-- Add fee column for transaction fees (preview feature)
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS fee DECIMAL(10,2) DEFAULT 0.00;

-- Backfill existing transactions with a default fee
UPDATE transactions SET fee = ROUND(amount * 0.015, 2) WHERE fee = 0.00;
