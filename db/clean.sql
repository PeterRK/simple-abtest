
DELETE FROM experiment WHERE app_id NOT IN (
	SELECT DISTINCT app_id FROM application
);

DELETE FROM exp_layer WHERE exp_id NOT IN (
	SELECT DISTINCT exp_id FROM experiment
);

DELETE FROM exp_segment WHERE lyr_id NOT IN (
	SELECT DISTINCT lyr_id FROM exp_layer
);

DELETE FROM exp_group WHERE seg_id NOT IN (
	SELECT DISTINCT seg_id FROM exp_segment
);

DELETE FROM exp_config WHERE grp_id NOT IN (
	SELECT DISTINCT grp_id FROM exp_group
);