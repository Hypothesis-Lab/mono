ALTER TABLE product
    ADD COLUMN custom_category VARCHAR(64);

UPDATE product
SET custom_category = 'Кредит в банке'
WHERE id = '2bb54b6a-6c11-4207-903f-7f27b164096d'
    AND category = 'other';

ALTER TABLE product
    ADD CONSTRAINT product_custom_category_check CHECK (
        (category = 'other' AND custom_category IS NOT NULL AND custom_category = BTRIM(custom_category) AND CHAR_LENGTH(custom_category) BETWEEN 1 AND 64)
        OR (category <> 'other' AND custom_category IS NULL)
    );
