CREATE TABLE IF NOT EXISTS messages (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id INT,
		nickname VARCHAR(255),
		message_content TEXT,
		message_time TIMESTAMP
)