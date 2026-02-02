-- Initial database schema for Mooncake PostgreSQL example
-- This creates a simple users table and adds sample data

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    full_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

-- Create index on username
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Create index on email
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Insert sample data
INSERT INTO users (username, email, full_name, is_active) VALUES
    ('alice', 'alice@example.com', 'Alice Johnson', true),
    ('bob', 'bob@example.com', 'Bob Smith', true),
    ('charlie', 'charlie@example.com', 'Charlie Brown', true),
    ('diana', 'diana@example.com', 'Diana Prince', false)
ON CONFLICT (username) DO NOTHING;

-- Create a simple posts table
CREATE TABLE IF NOT EXISTS posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on user_id
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);

-- Insert sample posts
INSERT INTO posts (user_id, title, content) VALUES
    (1, 'First Post', 'This is Alice''s first post!'),
    (1, 'Hello World', 'Hello from Alice!'),
    (2, 'Getting Started', 'Bob is learning PostgreSQL with Mooncake'),
    (3, 'Database Tips', 'Charlie shares some database tips')
ON CONFLICT DO NOTHING;

-- Create a view for active users
CREATE OR REPLACE VIEW active_users AS
SELECT
    id,
    username,
    email,
    full_name,
    created_at
FROM users
WHERE is_active = true;

-- Create a simple function
CREATE OR REPLACE FUNCTION get_user_post_count(user_id_param INTEGER)
RETURNS INTEGER AS $$
BEGIN
    RETURN (SELECT COUNT(*) FROM posts WHERE user_id = user_id_param);
END;
$$ LANGUAGE plpgsql;

-- Display summary
DO $$
DECLARE
    user_count INTEGER;
    post_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO user_count FROM users;
    SELECT COUNT(*) INTO post_count FROM posts;
    RAISE NOTICE 'Database initialized successfully!';
    RAISE NOTICE 'Created % users and % posts', user_count, post_count;
END $$;
