CREATE TABLE `phonebook` (
 `user_id` int(11) NOT NULL AUTO_INCREMENT,
 `phone_number` varchar(48) DEFAULT NULL,
 PRIMARY KEY (`user_id`),
 UNIQUE KEY `phone_number` (`phone_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_general_ci;

CREATE TABLE `un_assigned_numbers` (
 `phone_number` varchar(48) NOT NULL,
 `area_code` int(11) NOT NULL,
 `status` varchar(48) NOT NULL DEFAULT 'AVAILABLE',
 `ref_id` varchar(128) NOT NULL DEFAULT '',
 `timestamp` int(11) DEFAULT NULL COMMENT 'unix timestamp',
 INDEX `ref_id` (`ref_id`),
 INDEX `area_code` (`area_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_general_ci;

INSERT INTO `HUH8spzt3o`.`phonebook` (`phone_number`) 
VALUES (NULL), (NULL), (NULL), (NULL);

INSERT INTO `HUH8spzt3o`.`un_assigned_numbers` (`phone_number`, `area_code`) 
VALUES ('+16135550172', '613'), ('+16135550149', '613'), ('+16135550129', '613'), ('+16135550157', '613'),
('+16134550451', '613'), ('+16135251551', '613'), ('+16135150451', '613'), ('+16135141182', '613')
