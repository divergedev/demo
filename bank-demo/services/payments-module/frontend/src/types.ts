export interface PaymentSummary {
  total_payments: number;
  total_volume: number;
  total_fees?: number;
  flagged_count?: number;
}

export interface Transaction {
  id: string;
  from: string;
  to: string;
  amount: number;
  fee?: number;
  status: string;
  fraud_score?: number;
}
