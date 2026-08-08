import 'package:flutter/material.dart';
import '../core/theme/app_colors.dart';

/// A single EMI installment as returned by the backend
/// (`internal/models/emi.go` `EmiInstallment`), serialized by
/// `GET /auth/loans/:id/emi-schedule` and
/// `POST /auth/loans/:id/emi/:installmentId/pay`. Field names mirror the
/// backend's JSON tags exactly (camelCase) — see that struct before
/// changing anything here.
class EmiInstallment {
  final int id;
  final int loanId;
  final int installmentNumber;
  final DateTime dueDate;
  final double amount;
  final double principalComponent;
  final double interestComponent;
  final String status;
  final DateTime? paidAt;
  final DateTime createdAt;

  const EmiInstallment({
    required this.id,
    required this.loanId,
    required this.installmentNumber,
    required this.dueDate,
    required this.amount,
    required this.principalComponent,
    required this.interestComponent,
    required this.status,
    this.paidAt,
    required this.createdAt,
  });

  factory EmiInstallment.fromJson(Map<String, dynamic> json) {
    DateTime? parseDate(dynamic v) => v == null ? null : DateTime.tryParse(v as String);
    return EmiInstallment(
      id: json['id'] as int,
      loanId: json['loanId'] as int,
      installmentNumber: json['installmentNumber'] as int? ?? 0,
      dueDate: parseDate(json['dueDate']) ?? DateTime.now(),
      amount: (json['amount'] as num?)?.toDouble() ?? 0,
      principalComponent: (json['principalComponent'] as num?)?.toDouble() ?? 0,
      interestComponent: (json['interestComponent'] as num?)?.toDouble() ?? 0,
      status: json['status'] as String? ?? 'pending',
      paidAt: parseDate(json['paidAt']),
      createdAt: parseDate(json['createdAt']) ?? DateTime.now(),
    );
  }
}

/// Aggregated totals block nested inside the schedule response
/// (backend's `emiSummary`).
class EmiSummary {
  final int totalInstallments;
  final int paidCount;
  final int remainingCount;
  final double totalPaid;
  final double totalRemaining;
  final DateTime? nextDueDate;
  final double nextDueAmount;

  const EmiSummary({
    required this.totalInstallments,
    required this.paidCount,
    required this.remainingCount,
    required this.totalPaid,
    required this.totalRemaining,
    this.nextDueDate,
    required this.nextDueAmount,
  });

  factory EmiSummary.fromJson(Map<String, dynamic> json) {
    DateTime? parseDate(dynamic v) => v == null ? null : DateTime.tryParse(v as String);
    return EmiSummary(
      totalInstallments: json['totalInstallments'] as int? ?? 0,
      paidCount: json['paidCount'] as int? ?? 0,
      remainingCount: json['remainingCount'] as int? ?? 0,
      totalPaid: (json['totalPaid'] as num?)?.toDouble() ?? 0,
      totalRemaining: (json['totalRemaining'] as num?)?.toDouble() ?? 0,
      nextDueDate: parseDate(json['nextDueDate']),
      nextDueAmount: (json['nextDueAmount'] as num?)?.toDouble() ?? 0,
    );
  }
}

/// The full response of `GET /auth/loans/:id/emi-schedule`
/// (backend's `emiScheduleResponse`).
class EmiSchedule {
  final int loanId;
  final int tenureMonths;
  final double interestRatePercent;
  final List<EmiInstallment> installments;
  final EmiSummary summary;

  const EmiSchedule({
    required this.loanId,
    required this.tenureMonths,
    required this.interestRatePercent,
    required this.installments,
    required this.summary,
  });

  factory EmiSchedule.fromJson(Map<String, dynamic> json) {
    return EmiSchedule(
      loanId: json['loanId'] as int,
      tenureMonths: json['tenureMonths'] as int? ?? 0,
      interestRatePercent: (json['interestRatePercent'] as num?)?.toDouble() ?? 0,
      installments: (json['installments'] as List<dynamic>? ?? [])
          .map((e) => EmiInstallment.fromJson(e as Map<String, dynamic>))
          .toList(),
      summary: EmiSummary.fromJson(json['summary'] as Map<String, dynamic>? ?? const {}),
    );
  }
}

/// The dashboard-wide EMI aggregate returned by
/// `GET /auth/dashboard/emi-summary`. `hasUpcomingEmi: false` means the
/// customer has no active EMI schedule yet — all other fields are then
/// null/zero and must not be shown.
class EmiDashboardSummary {
  final bool hasUpcomingEmi;
  final DateTime? nextDueDate;
  final double nextDueAmount;
  final int? loanId;
  final double totalPaidAcrossLoans;
  final double totalRemainingAcrossLoans;
  final int totalActiveLoans;

  const EmiDashboardSummary({
    required this.hasUpcomingEmi,
    this.nextDueDate,
    required this.nextDueAmount,
    this.loanId,
    required this.totalPaidAcrossLoans,
    required this.totalRemainingAcrossLoans,
    required this.totalActiveLoans,
  });

  factory EmiDashboardSummary.fromJson(Map<String, dynamic> json) {
    DateTime? parseDate(dynamic v) => v == null ? null : DateTime.tryParse(v as String);
    return EmiDashboardSummary(
      hasUpcomingEmi: json['hasUpcomingEmi'] as bool? ?? false,
      nextDueDate: parseDate(json['nextDueDate']),
      nextDueAmount: (json['nextDueAmount'] as num?)?.toDouble() ?? 0,
      loanId: json['loanId'] as int?,
      totalPaidAcrossLoans: (json['totalPaidAcrossLoans'] as num?)?.toDouble() ?? 0,
      totalRemainingAcrossLoans: (json['totalRemainingAcrossLoans'] as num?)?.toDouble() ?? 0,
      totalActiveLoans: json['totalActiveLoans'] as int? ?? 0,
    );
  }
}

/// Maps the backend's raw EMI installment status strings
/// (`internal/models/emi.go`) to customer-friendly copy and a badge color
/// — mirrors `LoanStatusInfo` in `loan_application.dart` for consistency.
class EmiStatusInfo {
  const EmiStatusInfo(this.label, this.color);

  final String label;
  final Color color;

  static const _map = <String, EmiStatusInfo>{
    'paid': EmiStatusInfo('Paid', AppColors.success),
    'pending': EmiStatusInfo('Upcoming', AppColors.primary),
    'overdue': EmiStatusInfo('Overdue', AppColors.error),
  };

  static EmiStatusInfo forStatus(String status) =>
      _map[status] ?? EmiStatusInfo(status, AppColors.textSecondaryLight);
}
