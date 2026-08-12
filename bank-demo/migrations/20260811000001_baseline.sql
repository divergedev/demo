-- Create the baseline transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    from_account VARCHAR(20) NOT NULL,
    to_account VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Seed baseline transactions
INSERT INTO transactions (from_account, to_account, amount) VALUES
    ('ACC-001', 'ACC-002', 150.00),
    ('ACC-003', 'ACC-001', 75.50),
    ('ACC-002', 'ACC-004', 200.00),
    ('ACC-005', 'ACC-001', 50.00),
    ('ACC-001', 'ACC-003', 300.00),
    ('ACC-004', 'ACC-002', 125.75),
    ('ACC-002', 'ACC-005', 89.99),
    ('ACC-003', 'ACC-004', 175.00),
    ('ACC-001', 'ACC-005', 250.00),
    ('ACC-005', 'ACC-003', 60.25);
