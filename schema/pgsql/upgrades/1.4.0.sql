ALTER TABLE host ADD COLUMN total_children uint DEFAULT NULL;
ALTER TABLE host_state ADD COLUMN affects_children boolenum NOT NULL DEFAULT 'n';
ALTER TABLE host_state ALTER COLUMN affects_children DROP DEFAULT;

ALTER TABLE service ADD COLUMN total_children uint DEFAULT NULL;
ALTER TABLE service_state ADD COLUMN affects_children boolenum NOT NULL DEFAULT 'n';
ALTER TABLE service_state ALTER COLUMN affects_children DROP DEFAULT;

CREATE TABLE redundancy_group (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  display_name text NOT NULL,

  CONSTRAINT pk_redundancy_group PRIMARY KEY (id)
);

ALTER TABLE redundancy_group ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE redundancy_group ALTER COLUMN environment_id SET STORAGE PLAIN;

COMMENT ON COLUMN redundancy_group.id IS 'sha1(name + all(member parent_name + timeperiod.name + states + ignore_soft_states))';
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

ALTER TABLE redundancy_group_state ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE redundancy_group_state ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE redundancy_group_state ALTER COLUMN redundancy_group_id SET STORAGE PLAIN;

CREATE UNIQUE INDEX idx_redundancy_group_state_redundancy_group_id ON redundancy_group_state(redundancy_group_id);

COMMENT ON COLUMN redundancy_group_state.id IS 'redundancy_group.id';
COMMENT ON COLUMN redundancy_group_state.environment_id IS 'environment.id';
COMMENT ON COLUMN redundancy_group_state.redundancy_group_id IS 'redundancy_group.id';

CREATE TABLE dependency_node (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  host_id bytea20 DEFAULT NULL,
  service_id bytea20 DEFAULT NULL,
  redundancy_group_id bytea20 DEFAULT NULL,

  CONSTRAINT pk_dependency_node PRIMARY KEY (id),

  CONSTRAINT ck_dependency_node_either_checkable_or_redundancy_group_id CHECK (
    CASE WHEN redundancy_group_id IS NULL THEN host_id IS NOT NULL ELSE host_id IS NULL AND service_id IS NULL END
  )
);

ALTER TABLE dependency_node ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE dependency_node ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE dependency_node ALTER COLUMN host_id SET STORAGE PLAIN;
ALTER TABLE dependency_node ALTER COLUMN service_id SET STORAGE PLAIN;
ALTER TABLE dependency_node ALTER COLUMN redundancy_group_id SET STORAGE PLAIN;

CREATE UNIQUE INDEX idx_dependency_node_host_service_redundancygroup_id ON dependency_node(host_id, service_id, redundancy_group_id);

COMMENT ON COLUMN dependency_node.id IS 'host.id|service.id|redundancy_group.id';
COMMENT ON COLUMN dependency_node.environment_id IS 'environment.id';
COMMENT ON COLUMN dependency_node.host_id IS 'host.id';
COMMENT ON COLUMN dependency_node.service_id IS 'service.id';
COMMENT ON COLUMN dependency_node.redundancy_group_id IS 'redundancy_group.id';

CREATE TABLE dependency_edge_state (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  failed boolenum NOT NULL,

  CONSTRAINT pk_dependency_edge_state PRIMARY KEY (id)
);

ALTER TABLE dependency_edge_state ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE dependency_edge_state ALTER COLUMN environment_id SET STORAGE PLAIN;

COMMENT ON COLUMN dependency_edge_state.id IS 'sha1([dependency_edge.from_node_id|parent_name + timeperiod.name + states + ignore_soft_states] + dependency_edge.to_node_id)';
COMMENT ON COLUMN dependency_edge_state.environment_id IS 'environment.id';

CREATE TABLE dependency_edge (
  id bytea20 NOT NULL,
  environment_id bytea20 NOT NULL,
  from_node_id bytea20 NOT NULL,
  to_node_id bytea20 NOT NULL,
  dependency_edge_state_id bytea20 NOT NULL,
  display_name text NOT NULL,

  CONSTRAINT pk_dependency_edge PRIMARY KEY (id)
);

ALTER TABLE dependency_edge ALTER COLUMN id SET STORAGE PLAIN;
ALTER TABLE dependency_edge ALTER COLUMN environment_id SET STORAGE PLAIN;
ALTER TABLE dependency_edge ALTER COLUMN from_node_id SET STORAGE PLAIN;
ALTER TABLE dependency_edge ALTER COLUMN to_node_id SET STORAGE PLAIN;
ALTER TABLE dependency_edge ALTER COLUMN dependency_edge_state_id SET STORAGE PLAIN;

CREATE UNIQUE INDEX idx_dependency_edge_from_node_to_node_id ON dependency_edge(from_node_id, to_node_id);

COMMENT ON COLUMN dependency_edge.id IS 'sha1(from_node_id + to_node_id)';
COMMENT ON COLUMN dependency_edge.environment_id IS 'environment.id';
COMMENT ON COLUMN dependency_edge.from_node_id IS 'dependency_node.id';
COMMENT ON COLUMN dependency_edge.to_node_id IS 'dependency_node.id';
COMMENT ON COLUMN dependency_edge.dependency_edge_state_id IS 'sha1(dependency_edge_state.id)';

ALTER TABLE icingadb_instance ADD COLUMN icingadb_version varchar(255) NOT NULL DEFAULT 'unknown';
ALTER TABLE icingadb_instance ALTER COLUMN icingadb_version DROP DEFAULT;

INSERT INTO icingadb_schema (version, timestamp)
  VALUES (5, extract(epoch from now()) * 1000);
