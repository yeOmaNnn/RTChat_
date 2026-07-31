package storage 

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool 
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("создание пула pgx: %w", err) 
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err) 
	}

	return &Store{pool : pool}, nil 
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) SaveMessage(ctx context.Context, m Message) (Message, error) {
	row := s.pool.QueryRow(ctx, `INSERT INTO messages(room_id, username, content)
								 VALUES ($1, $2, $3)
								 RETURNING id, created_at
								`, m.RoomID, m.Username, m.Content,)
	if err := row.Scan(&m.ID, &m.CreateAt); err != nil {
		return Message{}, fmt.Errorf("вставка сообщения: %w", err)
	}
	return m, nil 
}

func (s *Store) History(ctx context.Context, roomID string, limit int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `
									SELECT id, room_id, username, content, created_at
									FROM messages
									WHERE room_id = $1
									ORDER_BY created_at DESC 
									LIMIT $2
									`, roomID, limit,)
	if err != nil {
		return nil, fmt.Errorf("вывод истории сообщении: %w", err)
	}
	defer rows.Close()

	var msgs []Message 
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.Username, &m.Content, &m.CreateAt); err != nil {
			return nil, fmt.Errorf("Поиск истории сообщении: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err 
	}

	for i, j := 0, len(msgs) - 1; i < j; i, j = i + 1, j - 1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil 
}

