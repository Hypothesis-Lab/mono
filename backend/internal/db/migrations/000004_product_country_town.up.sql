ALTER TABLE product
    ADD COLUMN country VARCHAR(255),
    ADD COLUMN town VARCHAR(255);

UPDATE product
SET
    country = 'Россия',
    town = geography;

ALTER TABLE product
    ALTER COLUMN country SET NOT NULL,
    DROP COLUMN geography;
