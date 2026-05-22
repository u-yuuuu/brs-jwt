CREATE TABLE IF NOT EXISTS usertypes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type_id INTEGER REFERENCES usertypes (id),
    username TEXT UNIQUE,
    password TEXT NOT NULL,
    github_id TEXT UNIQUE
);

CREATE TABLE IF NOT EXISTS subjects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS courses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER REFERENCES subjects (id)
);

CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS student_in_group (
    user_id INTEGER REFERENCES users (id) UNIQUE,
    group_id INTEGER REFERENCES groups (id)
);

CREATE TABLE IF NOT EXISTS course_admins (
    user_id INTEGER REFERENCES users (id),
    course_id INTEGER REFERENCES courses (id)
);

CREATE TABLE IF NOT EXISTS subject_attributes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER REFERENCES subjects (id),
    name TEXT,
    max_points REAL
);

CREATE TABLE IF NOT EXISTS students_points (
    user_id REFERENCES courses (id),
    course_id INTEGER REFERENCES courses (id),
    attribute_id INTEGER REFERENCES subject_attributes(id),
    point REAL
);

CREATE TABLE IF NOT EXISTS course_attendees (
    course_id INTEGER REFERENCES courses (id),
    group_id INTEGER REFERENCES groups (id)
);

CREATE TABLE IF NOT EXISTS users_info (
    user_id REFERENCES courses (id) UNIQUE,
    last_name TEXT,
    first_name TEXT
);

CREATE TABLE IF NOT EXISTS oauth_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    state TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    used BOOLEAN DEFAULT 0,
    user_id INTEGER REFERENCES users(id)
);

