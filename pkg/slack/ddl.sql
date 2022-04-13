-- clean
DROP TABLE IF EXISTS kitten.installation;

DROP INDEX IF EXISTS installation_id;

DROP SCHEMA IF EXISTS kitten;

-- schema
CREATE SCHEMA kitten;

-- installation
CREATE TABLE kitten.installation (
  enterprise_id TEXT NOT NULL,
  team_id TEXT NOT NULL,
  scope TEXT NOT NULL,
  access_token TEXT NOT NULL,
  creation_date TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE UNIQUE INDEX workspaces_id ON kitten.installation(enterprise_id, team_id);
