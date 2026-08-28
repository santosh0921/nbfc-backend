import 'api_client.dart';
import '../../models/emi_schedule.dart';

/// Wraps the backend's protected EMI endpoints
/// (`internal/loans/emi_handler.go`, `internal/payments`) — the full
/// amortization schedule for a loan, starting a real gateway payment
/// order, polling its verified status, and the cross-loan dashboard EMI
/// summary. There is no "mark installment paid" call here anymore — only
/// the backend's webhook handler can do that, after verifying the
/// gateway's own confirmation.
class EmiApiService {
  EmiApiService._();

  static Future<EmiSchedule> schedule(String token, int loanId) async {
    final res = await ApiClient.get('/auth/loans/$loanId/emi-schedule', token: token);
    return EmiSchedule.fromJson(res);
  }

  /// Starts a real gateway payment order for one installment. Throws
  /// [ApiException] (503) if no payment gateway is configured on the
  /// backend.
  static Future<PaymentOrder> createPaymentOrder(String token, int loanId, int installmentId) async {
    final res = await ApiClient.post('/auth/loans/$loanId/emi/$installmentId/create-order', const {}, token: token);
    return PaymentOrder.fromJson(res);
  }

  /// Polls the backend-verified status of a payment — never set by the
  /// client, only ever read.
  static Future<PaymentStatus> paymentStatus(String token, int paymentId) async {
    final res = await ApiClient.get('/auth/payments/$paymentId', token: token);
    return PaymentStatus.fromJson(res);
  }

  static Future<EmiDashboardSummary> dashboardSummary(String token) async {
    final res = await ApiClient.get('/auth/dashboard/emi-summary', token: token);
    return EmiDashboardSummary.fromJson(res);
  }

  /// Demo-mode-only settlement (`internal/loans/emi_handler.go`'s
  /// CustomerDemoPayEmiHandler) — 404s if the backend isn't running with
  /// DEMO_MODE set. Unlike a real gateway payment, this genuinely marks
  /// the installment paid server-side (accounting posting, receipt
  /// generation, the works) without a gateway in the loop, so the demo
  /// app's own EMI/payment-history views reflect a real state change
  /// instead of a UI-only success screen. Returns the updated schedule.
  static Future<EmiSchedule> demoPay(String token, int loanId, int installmentId) async {
    final res = await ApiClient.post('/auth/loans/$loanId/emi/$installmentId/demo-pay', const {}, token: token);
    return EmiSchedule.fromJson(res);
  }
}
