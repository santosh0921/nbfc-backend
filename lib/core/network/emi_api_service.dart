import 'api_client.dart';
import '../../models/emi_schedule.dart';

/// Wraps the backend's protected EMI endpoints
/// (`internal/loans/emi_handler.go`) — the full amortization schedule for
/// a loan, marking an installment paid (mock payment), and the
/// cross-loan dashboard EMI summary.
class EmiApiService {
  EmiApiService._();

  static Future<EmiSchedule> schedule(String token, int loanId) async {
    final res = await ApiClient.get('/auth/loans/$loanId/emi-schedule', token: token);
    return EmiSchedule.fromJson(res);
  }

  static Future<EmiInstallment> payInstallment(String token, int loanId, int installmentId) async {
    final res = await ApiClient.post('/auth/loans/$loanId/emi/$installmentId/pay', const {}, token: token);
    return EmiInstallment.fromJson(res);
  }

  static Future<EmiDashboardSummary> dashboardSummary(String token) async {
    final res = await ApiClient.get('/auth/dashboard/emi-summary', token: token);
    return EmiDashboardSummary.fromJson(res);
  }
}
