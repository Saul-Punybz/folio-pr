-- Add image_url column to articles for storing article thumbnail/preview images.
ALTER TABLE articles ADD COLUMN IF NOT EXISTS image_url TEXT;
