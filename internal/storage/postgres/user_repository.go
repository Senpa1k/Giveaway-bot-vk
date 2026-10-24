package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// statementBuilder is the shared dollar-placeholder SQL builder.
func statementBuilder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}

// UserRepository stores bot users and their dialog states.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a user repository over a connection pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// UpsertByVKID registers the first message of a VK user and returns the user.
// The conflict update only refreshes updated_at, so concurrent first messages
// converge on a single row.
func (r *UserRepository) UpsertByVKID(ctx context.Context, vkID int64) (*model.User, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (vk_id) VALUES ($1)
		ON CONFLICT (vk_id) DO UPDATE SET updated_at = now()
		RETURNING id, vk_id, first_name, last_name, state, state_ctx, created_at, updated_at`,
		vkID,
	)
	return scanUser(row)
}

// ByID returns the user or model.ErrUserNotFound.
func (r *UserRepository) ByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, vk_id, first_name, last_name, state, state_ctx, created_at, updated_at
		FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// ByIDs returns the users with the given IDs; missing IDs are omitted.
func (r *UserRepository) ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]model.User, error) {
	users := make(map[uuid.UUID]model.User, len(ids))
	if len(ids) == 0 {
		return users, nil
	}

	query, args, err := statementBuilder().
		Select("id", "vk_id", "first_name", "last_name", "state", "state_ctx", "created_at", "updated_at").
		From("users").
		Where(sq.Eq{"id": ids}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build users query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users[user.ID] = *user
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}
	return users, nil
}

// SetState persists the dialog state of the user.
func (r *UserRepository) SetState(ctx context.Context, id uuid.UUID, state model.UserState) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET state = $2, state_ctx = $3, updated_at = now()
		WHERE id = $1`,
		id, state.State, stateContext(state),
	)
	if err != nil {
		return fmt.Errorf("set user state: %w", err)
	}
	return nil
}

// AllVKIDs lists VK ids of every user for broadcasts.
func (r *UserRepository) AllVKIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT vk_id FROM users ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("select user vk ids: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var vkID int64
		if err := rows.Scan(&vkID); err != nil {
			return nil, fmt.Errorf("scan user vk id: %w", err)
		}
		ids = append(ids, vkID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read user vk ids: %w", err)
	}
	return ids, nil
}

// scanUser reads one user row from a row or rows value.
func scanUser(row pgx.Row) (*model.User, error) {
	var user model.User
	var state string
	var stateCtx []byte
	if err := row.Scan(
		&user.ID,
		&user.VKID,
		&user.FirstName,
		&user.LastName,
		&state,
		&stateCtx,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}

	user.State = parseState(state, stateCtx)
	return &user, nil
}

// parseState decodes the dialog state columns; a broken context degrades to
// the plain state without failing the read.
func parseState(state string, stateCtx []byte) model.UserState {
	result := model.UserState{State: state}
	if len(stateCtx) == 0 {
		return result
	}
	var ctx struct {
		WinnerID string `json:"winner_id"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal(stateCtx, &ctx); err != nil {
		return result
	}
	if id, err := uuid.Parse(ctx.WinnerID); err == nil {
		result.ID = id
	}
	result.Name = ctx.Name
	return result
}

// stateContext encodes the claim-flow context of a dialog state.
func stateContext(state model.UserState) []byte {
	if state.ID == uuid.Nil && state.Name == "" {
		return []byte("{}")
	}
	ctx := make(map[string]string, 2)
	if state.ID != uuid.Nil {
		ctx["winner_id"] = state.ID.String()
	}
	if state.Name != "" {
		ctx["name"] = state.Name
	}
	encoded, err := json.Marshal(ctx)
	if err != nil {
		return []byte("{}")
	}
	return encoded
}
