package store

import (
	"encoding/json"
	"time"
)

type Provider struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	BaseURL   string    `json:"base_url"`
	Models    []string  `json:"models"`
	IsActive  bool      `json:"is_active"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ps *PostgresStore) CreateProvider(p Provider) (Provider, error) {
	modelsJSON, _ := json.Marshal(p.Models)

	err := ps.db.QueryRow(`
		INSERT INTO providers (name, api_key, base_url, models, is_active, priority)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, p.Name, p.APIKey, p.BaseURL, modelsJSON, p.IsActive, p.Priority).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	return p, err
}

func (ps *PostgresStore) ListProviders() ([]Provider, error) {
	rows, err := ps.db.Query(`
		SELECT id, name, api_key, base_url, models, is_active, priority, created_at, updated_at
		FROM providers
		ORDER BY priority ASC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []Provider
	for rows.Next() {
		var p Provider
		var modelsJSON []byte
		err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &modelsJSON,
			&p.IsActive, &p.Priority, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(modelsJSON, &p.Models)
		providers = append(providers, p)
	}

	return providers, nil
}

func (ps *PostgresStore) GetProvider(id string) (Provider, error) {
	var p Provider
	var modelsJSON []byte

	err := ps.db.QueryRow(`
		SELECT id, name, api_key, base_url, models, is_active, priority, created_at, updated_at
		FROM providers WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &p.APIKey, &p.BaseURL, &modelsJSON,
		&p.IsActive, &p.Priority, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return p, err
	}

	json.Unmarshal(modelsJSON, &p.Models)
	return p, nil
}

func (ps *PostgresStore) UpdateProvider(id string, p Provider) error {
	modelsJSON, _ := json.Marshal(p.Models)

	_, err := ps.db.Exec(`
		UPDATE providers
		SET name = $1, api_key = $2, base_url = $3, models = $4,
		    is_active = $5, priority = $6, updated_at = NOW()
		WHERE id = $7
	`, p.Name, p.APIKey, p.BaseURL, modelsJSON, p.IsActive, p.Priority, id)

	return err
}

func (ps *PostgresStore) DeleteProvider(id string) error {
	_, err := ps.db.Exec(`DELETE FROM providers WHERE id = $1`, id)
	return err
}

func (ps *PostgresStore) SwitchActiveProvider(id string) error {
	tx, err := ps.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 将所有 provider 设为 inactive
	_, err = tx.Exec(`UPDATE providers SET is_active = false`)
	if err != nil {
		return err
	}

	// 将指定 provider 设为 active
	_, err = tx.Exec(`UPDATE providers SET is_active = true WHERE id = $1`, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}
