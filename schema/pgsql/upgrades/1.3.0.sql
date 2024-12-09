ALTER TABLE host ALTER COLUMN icon_image_alt TYPE text;
ALTER TABLE service ALTER COLUMN icon_image_alt TYPE text;

COMMENT ON COLUMN endpoint.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN comment.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN notification.properties_checksum IS 'sha1(all properties)';

ALTER TABLE timeperiod_range ALTER COLUMN range_value TYPE text;

ALTER TABLE checkcommand_argument ALTER COLUMN argument_key TYPE varchar(255);
ALTER TABLE eventcommand_argument ALTER COLUMN argument_key TYPE varchar(255);
ALTER TABLE notificationcommand_argument ALTER COLUMN argument_key TYPE varchar(255);

ALTER TABLE checkcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);
ALTER TABLE eventcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);
ALTER TABLE notificationcommand_envvar ALTER COLUMN envvar_key TYPE varchar(255);

ALTER TABLE host ADD COLUMN affected_children uint DEFAULT NULL;
ALTER TABLE host_state ADD COLUMN affects_children boolenum DEFAULT 'n';
ALTER TABLE host_state ALTER COLUMN affects_children DROP DEFAULT;

ALTER TABLE service ADD COLUMN affected_children uint DEFAULT NULL;
ALTER TABLE service_state ADD COLUMN affects_children boolenum DEFAULT 'n';
ALTER TABLE service_state ALTER COLUMN affects_children DROP DEFAULT;

CREATE TABLE redundancy_group (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name text NOT NULL,

  CONSTRAINT pk_redundancy_group PRIMARY KEY (id)
);

COMMENT ON COLUMN redundancy_group.id IS 'sha1(name + all-member-parent-names)';
COMMENT ON COLUMN redundancy_group.environment_id IS 'environment.id';

CREATE TABLE redundancy_group_state (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  redundancy_group_id bytea20 NOT NULL,
  failed boolenum NOT NULL,
  is_reachable boolenum NOT NULL,
  last_state_change biguint NOT NULL,

  CONSTRAINT pk_redundancy_group_state PRIMARY KEY (id)
);

CREATE UNIQUE INDEX idx_redundancy_group_state_redundancy_group_id ON redundancy_group_state(redundancy_group_id);

COMMENT ON COLUMN redundancy_group_state.id IS 'redundancy_group.id';
COMMENT ON COLUMN redundancy_group_state.environment_id IS 'environment.id';
COMMENT ON COLUMN redundancy_group_state.redundancy_group_id IS 'redundancy_group.id';

CREATE TABLE dependency (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  name text NOT NULL,
  display_name text NOT NULL,
  name_checksum bytea20 NOT NULL,
  properties_checksum bytea20 NOT NULL,
  redundancy_group_id bytea20 DEFAULT NULL,
  timeperiod_id bytea20 DEFAULT NULL,
  disable_checks boolenum NOT NULL,
  disable_notifications boolenum NOT NULL,
  ignore_soft_states boolenum NOT NULL,
  states tinyuint NOT NULL,

  CONSTRAINT pk_dependency PRIMARY KEY (id)
);

COMMENT ON COLUMN dependency.id IS 'sha1(environment.id + name)';
COMMENT ON COLUMN dependency.environment_id IS 'environment.id';
COMMENT ON COLUMN dependency.name_checksum IS 'sha1(name)';
COMMENT ON COLUMN dependency.properties_checksum IS 'sha1(all properties)';
COMMENT ON COLUMN dependency.redundancy_group_id IS 'redundancy_group.id';
COMMENT ON COLUMN dependency.timeperiod_id IS 'timeperiod.id';

CREATE TABLE dependency_state (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  dependency_id bytea20 NOT NULL,
  failed boolenum NOT NULL,

  CONSTRAINT pk_dependency_state PRIMARY KEY (id)
);

CREATE UNIQUE INDEX idx_dependency_state_dependency_id ON dependency_state(dependency_id);

COMMENT ON COLUMN dependency_state.id IS 'dependency.id';
COMMENT ON COLUMN dependency_state.environment_id IS 'environment.id';
COMMENT ON COLUMN dependency_state.dependency_id IS 'dependency.id';

CREATE TABLE dependency_node (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  host_id bytea20 DEFAULT NULL,
  service_id bytea20 DEFAULT NULL,
  redundancy_group_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_dependency_node PRIMARY KEY (id),

  CONSTRAINT ck_dependency_node_either_checkable_or_redundancy_group_id CHECK (num_nonnulls(host_id, service_id, redundancy_group_id) <= 2)
);

CREATE UNIQUE INDEX idx_dependency_node_host_service_redundancygroup_id ON dependency_node(host_id, service_id, redundancy_group_id);

COMMENT ON COLUMN dependency_node.id IS 'host.id|service.id|redundancy_group.id';
COMMENT ON COLUMN dependency_node.environment_id IS 'environment.id';
COMMENT ON COLUMN dependency_node.host_id IS 'host.id';
COMMENT ON COLUMN dependency_node.service_id IS 'service.id';
COMMENT ON COLUMN dependency_node.redundancy_group_id IS 'redundancy_group.id';

CREATE TABLE dependency_edge (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  from_node_id bytea20 NOT NULL,
  to_node_id bytea20 NOT NULL,
  dependency_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_dependency_edge PRIMARY KEY (id)
);

CREATE UNIQUE INDEX idx_dependency_edge_from_node_to_node_dependency_id ON dependency_edge(from_node_id, to_node_id, dependency_id);

COMMENT ON COLUMN dependency_edge.id IS 'sha1(from_node_id + to_node_id + [dependency.id])';
COMMENT ON COLUMN dependency_edge.environment_id IS 'environment.id';
COMMENT ON COLUMN dependency_edge.from_node_id IS 'host.id|service.id|redundancy_group.id';
COMMENT ON COLUMN dependency_edge.to_node_id IS 'host.id|service.id|redundancy_group.id';
COMMENT ON COLUMN dependency_edge.dependency_id IS 'dependency.id';

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (4, extract(epoch from now()) * 1000);
