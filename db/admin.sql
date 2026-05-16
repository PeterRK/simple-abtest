CREATE TABLE IF NOT EXISTS application (
	app_id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
	name VARCHAR(64) NOT NULL,
	description VARCHAR(255) NOT NULL,
	
	access_token CHAR(24) NOT NULL DEFAULT '',

	version INT UNSIGNED NOT NULL DEFAULT 0,
	update_time DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user (
	uid INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
	name VARCHAR(64) NOT NULL,
	slat BINARY(16) NOT NULL,
	password BINARY(32) NOT NULL,	-- sha256
	
	update_time DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	
	UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS privilege (
	uid INT UNSIGNED NOT NULL,
	app_id INT UNSIGNED NOT NULL,
    privilege TINYINT UNSIGNED NOT NULL, -- 1=READ_ONLY,2=READ_WRITE,3=ADMIN
    grant_by INT UNSIGNED NOT NULL,

    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (uid, app_id),
    INDEX (app_id)
);

CREATE TABLE IF NOT EXISTS exp_result (
	_id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
	app_id INT UNSIGNED NOT NULL,
	exp_id INT UNSIGNED NOT NULL,
	layer_name VARCHAR(32) NOT NULL,
	group_name VARCHAR(32) NOT NULL,
	metric_name VARCHAR(128) NOT NULL,
	bucket_type VARCHAR(16) NOT NULL,
	bucket_key VARCHAR(64) NOT NULL,
	bucket_stamp BIGINT NOT NULL,
	metric_value DOUBLE NOT NULL,

	update_time DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

	UNIQUE KEY uniq_exp_result (
		app_id,
		exp_id,
		layer_name,
		bucket_type,
		metric_name,
		bucket_key,
		group_name
	),
	INDEX idx_exp_result_report (
		app_id,
		exp_id,
		layer_name,
		bucket_type,
		metric_name,
		bucket_stamp
	)
);
