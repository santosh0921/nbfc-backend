enum LoanStatus { active, closed, overdue, pendingDisbursal }

class ActiveLoan {
  final String loanId;
  final String productName;
  final double principal;
  final double outstanding;
  final double nextEmiAmount;
  final DateTime nextEmiDate;
  final int tenureMonths;
  final int monthsPaid;
  final double interestRate;
  final LoanStatus status;

  const ActiveLoan({
    required this.loanId,
    required this.productName,
    required this.principal,
    required this.outstanding,
    required this.nextEmiAmount,
    required this.nextEmiDate,
    required this.tenureMonths,
    required this.monthsPaid,
    required this.interestRate,
    required this.status,
  });

  double get progress => monthsPaid / tenureMonths;
}
