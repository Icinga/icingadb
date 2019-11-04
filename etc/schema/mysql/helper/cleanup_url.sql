-- IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

DROP PROCEDURE IF EXISTS cleanup_url;
DELIMITER //
CREATE PROCEDURE cleanup_url()
  BEGIN
    DELETE
    FROM action_url
    WHERE NOT EXISTS(
      SELECT h.action_url_id
      FROM host h
      WHERE h.action_url_id = action_url.id
    );

    DELETE
    FROM notes_url
    WHERE NOT EXISTS(
      SELECT h.notes_url_id
      FROM host h
      WHERE h.notes_url_id = notes_url.id
    );

    DELETE
    FROM icon_image
    WHERE NOT EXISTS(
      SELECT h.icon_image_id
      FROM host h
      WHERE h.icon_image_id = icon_image.id
    );
  END//
DELIMITER ;
