INSERT INTO users (type_id, username, password) 
SELECT id, ?, ? 
FROM usertypes 
WHERE name = ?;