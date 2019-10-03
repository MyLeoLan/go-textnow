CREATE TABLE `phonebook` (
 `user_id` int(11) NOT NULL AUTO_INCREMENT,
 `phone_number` varchar(48) DEFAULT NULL,
 PRIMARY KEY (`user_id`),
 UNIQUE KEY `phone_number` (`phone_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_general_ci;

INSERT INTO `HUH8spzt3o`.`phonebook` (`phone_number`) 
VALUES (NULL), (NULL), (NULL), (NULL);