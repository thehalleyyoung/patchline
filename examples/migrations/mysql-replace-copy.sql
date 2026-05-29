REPLACE INTO `users` (id, email) VALUES (1, 'a@example.com');

ALTER TABLE `users` ADD COLUMN tier varchar(10), ALGORITHM=COPY;
