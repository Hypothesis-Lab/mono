ALTER TABLE product
    ADD COLUMN geography VARCHAR(255);

UPDATE product
SET geography = COALESCE(town, country);

ALTER TABLE product
    ALTER COLUMN geography SET NOT NULL,
    DROP COLUMN town,
    DROP COLUMN country;
