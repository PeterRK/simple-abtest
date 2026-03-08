
DELETE FROM experiment WHERE app_id NOT IN (
	SELECT app_id FROM application
);

DELETE FROM exp_layer WHERE exp_id NOT IN (
	SELECT exp_id FROM experiment
);

DELETE FROM exp_segment WHERE lyr_id NOT IN (
	SELECT lyr_id FROM exp_layer
);

DELETE FROM exp_group WHERE seg_id NOT IN (
	SELECT seg_id FROM exp_segment
);

DELETE FROM exp_config WHERE grp_id NOT IN (
	SELECT grp_id FROM exp_group
);

DELETE FROM privilege WHERE uid NOT IN (
	SELECT uid FROM user
);

DELETE FROM privilege WHERE app_id NOT IN (
	SELECT app_id FROM application
);