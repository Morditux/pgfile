# pgfile

`pgfile` est une bibliothèque Go permettant de stocker des fichiers dans une base de données PostgreSQL en utilisant `pgx`.

## Caractéristiques

- **Chunking** : Les fichiers sont découpés en morceaux de 4 ko (4096 octets) et stockés dans un champ `bytea`.
- **Habilitation** : Gestion des accès basée sur des UUID de groupe et d'utilisateur.
- **Métadonnées** : Stockage du nom, du chemin, et des dates de création/modification.
- **Performance** : Utilise `pgxpool` pour une gestion efficace des connexions.

## Installation

```bash
go get github.com/Morditux/pgfile
```

## Schéma de Base de Données

Vous devez initialiser votre base de données avec le schéma suivant :

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    group_id UUID NOT NULL,
    owner_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE file_chunks (
    file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    sequence INT NOT NULL,
    data BYTEA NOT NULL,
    PRIMARY KEY (file_id, sequence)
);
```

## Utilisation

### Initialisation

```go
pool, err := pgxpool.New(context.Background(), connString)
storage := pgfile.NewStorage(pool)
```

### Upload d'un fichier

```go
file, err := os.Open("image.png")
meta, err := storage.Upload(ctx, "image.png", "/photos/2026", groupID, userID, file)
```

### Téléchargement d'un fichier

```go
err := storage.Download(ctx, fileID, userID, groupID, outWriter)
```

## Licence

MIT
