package slack

import (
	"context"

	"github.com/jackc/pgx/v4"
)

// Database interface needed
type Database interface {
	Enabled() bool
	Get(context.Context, func(pgx.Row) error, string, ...any) error
	One(context.Context, string, ...any) error
	DoAtomic(context.Context, func(context.Context) error) error
}

const getByIDQuery = `
SELECT
  access_token
FROM
  kitten.installation
WHERE
  enterprise_id = $1
  AND team_id = $2
`

// GetToken get token of an installation
func (a App) GetToken(ctx context.Context, entrepriseID, teamID string) (string, error) {
	var token string
	scanner := func(row pgx.Row) error {
		return row.Scan(&token)
	}

	return token, a.db.Get(ctx, scanner, getByIDQuery, entrepriseID, teamID)
}

const insertQuery = `
INSERT INTO
  kitten.installation
(
  enterprise_id,
  team_id,
  scope,
  access_token
) VALUES (
  $1,
  $2,
  $3,
  $4
)
`

// Create an installation
func (a App) Create(ctx context.Context, installation slackOauthReponse) error {
	return a.db.One(ctx, insertQuery, installation.Enterprise.ID, installation.Team.ID, installation.Scope, installation.AccessToken)
}

const updateQuery = `
UPDATE
  kitten.installation
SET
  scope = $3,
  access_token = $4
WHERE
  enterprise_id = $1
  AND team_id = $2
`

// Update an installation
func (a App) Update(ctx context.Context, installation slackOauthReponse) error {
	return a.db.One(ctx, updateQuery, installation.Enterprise.ID, installation.Team.ID, installation.Scope, installation.AccessToken)
}
