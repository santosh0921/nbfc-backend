import '../../models/active_loan.dart';
import '../network/emi_api_service.dart';
import '../network/loan_api_service.dart';

/// Builds a real [ActiveLoan] — the customer's actual next-due EMI plus
/// that loan's own category/amount/tenure — from the backend, shared by
/// every screen that used to show `MockData.activeLoans.first` instead.
/// Returns null if the customer has no upcoming EMI to pay.
class ActiveLoanLoader {
  ActiveLoanLoader._();

  static Future<ActiveLoan?> load(String token) async {
    final summary = await EmiApiService.dashboardSummary(token);
    if (!summary.hasUpcomingEmi || summary.loanId == null || summary.nextDueDate == null) return null;

    final loanId = summary.loanId!;
    final detail = await LoanApiService.detail(token, loanId);
    final schedule = await EmiApiService.schedule(token, loanId);

    return ActiveLoan(
      loanId: 'NBFC-APP-$loanId',
      productName: detail.category,
      principal: detail.amountRequested,
      outstanding: summary.totalRemainingAcrossLoans,
      nextEmiAmount: summary.nextDueAmount,
      nextEmiDate: summary.nextDueDate!,
      tenureMonths: schedule.tenureMonths,
      monthsPaid: schedule.installments.where((i) => i.status == 'paid').length,
      interestRate: schedule.interestRatePercent,
      status: LoanStatus.active,
    );
  }
}
