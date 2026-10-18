-- name: CreateConversation :one
INSERT INTO conversations (id)
VALUES (COALESCE($1::uuid, gen_random_uuid()))
RETURNING id, created_at;

-- name: InsertMessage :one
INSERT INTO messages (conversation_id, role, content)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, role, content, created_at;

-- name: ListLastMessagesByConversation :many
SELECT id, conversation_id, role, content, created_at
FROM (
    SELECT id, conversation_id, role, content, created_at
    FROM messages
    WHERE conversation_id = $1
    ORDER BY created_at DESC
    LIMIT $2
) sub
ORDER BY created_at ASC;
