START TRANSACTION;-- Transaction for safety.

--  Create the database schema for Cinnamon.
CREATE TABLE IF NOT EXISTS sshkeys (
  id INT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  identifier TEXT NOT NULL UNIQUE,
  keystring TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL
);-- Key Schema

CREATE TABLE IF NOT EXISTS users (
  id INT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  username TEXT NOT NULL UNIQUE,
  nickname TEXT NULL,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_login TIMESTAMP NULL
  deleted_at TIMESTAMP NULL
);-- User Schema

CREATE TABLE IF NOT EXISTS hashes (
  id INT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  user_id INT REFERENCES users (id),
  pw_hash TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL
); -- User PW Hash Schema

COMMIT;-- Commit the transaction.

-- It is recommended to also insert the private key for the server into the sshkeys table

INSERT INTO sshkeys (identifier, keystring) VALUES ('localhost', 'ssh-rsa AAAAB...');