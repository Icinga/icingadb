-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+


DELIMITER //
CREATE PROCEDURE populate_pending_hosts_for_environment(
  IN environment_id BINARY(20)
)
  BEGIN
    SET @bigNow = unix_timestamp_ms();
    INSERT INTO host_state (
      host_id,
      environment_id,
      state_type,
      soft_state,
      hard_state,
      attempt,
      is_active_check,
      is_problem,
      is_handled,
      is_reachable,
      is_flapping,
      is_acknowledged,
      in_downtime,
      last_update,
      last_state_change,
      last_soft_state,
      last_hard_state,
      next_check
    ) SELECT
        h.id,
        environment_id,
        'hard',
        99,
        99,
        1,
        'n',
        'n',
        'n',
        'y',
        'n',
        'n',
        'n',
        @bigNow,
        @bigNow,
        99,
        99,
        @bigNow + h.check_interval * 1000
      FROM host h
      WHERE NOT EXISTS(
          SELECT host_id
          FROM host_state hs
          WHERE hs.host_id = h.id)
            AND h.environment_id = environment_id;

  END

//
DELIMITER ;
