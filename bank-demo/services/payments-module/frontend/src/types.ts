export interface PaymentSummary {
  service: string;
  version: string;
  total_payments: number;
  total_volume: number;
  total_fees?: number;
  flagged_count: number;
  fraud_detection: boolean;
}

export interface Transaction {
  id: string;
  from: string;
  to: string;
  amount: number;
  fee?: number;
  status: string;
  fraud_score?: number;
  fraud_flag?: boolean;
  fraud_reason?: string;
}
