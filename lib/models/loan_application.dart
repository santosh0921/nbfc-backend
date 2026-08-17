/// A masked loan reference as returned by the backend
/// (`internal/loans/reference.go` `MaskedReference`) — included in any
/// full loan-detail response. Raw Aadhaar/PAN are never sent to the
/// client, only masked forms (`aadhaarMasked`: "XXXX-XXXX-1234",
/// `panMasked`: "******1234").
class LoanReferenceInfo {
  const LoanReferenceInfo({
    required this.id,
    required this.name,
    required this.relation,
    required this.phone,
    required this.aadhaarMasked,
    required this.panMasked,
  });

  final int id;
  final String name;
  final String relation;
  final String phone;
  final String aadhaarMasked;
  final String panMasked;

  factory LoanReferenceInfo.fromJson(Map<String, dynamic> json) => LoanReferenceInfo(
        id: json['id'] as int? ?? 0,
        name: json['name'] as String? ?? '',
        relation: json['relation'] as String? ?? '',
        phone: json['phone'] as String? ?? '',
        aadhaarMasked: json['aadhaarMasked'] as String? ?? '',
        panMasked: json['panMasked'] as String? ?? '',
      );
}

/// A customer's loan application as returned by the backend
/// (`internal/models/loan.go` `LoanApplication`, serialized by
/// `GET /auth/loans/mine` and `GET /auth/loans/:id`). Field names mirror
/// the backend's JSON tags exactly (camelCase) — see that struct before
/// changing anything here.
class LoanApplication {
  final int id;
  final String category;
  final double amountRequested;
  final String purpose;
  final String applicantName;
  final String applicantPhone;
  final String addressLine;
  final String city;
  final String status;
  final bool autoVerified;
  final String? autoVerifiedNote;
  final int? assignedEmployeeId;
  final String? verificationReport;
  final String? verificationFindings;
  final String? verificationRisk;
  final String? verificationRecommendation;
  final String? decisionRemarks;
  final String? decidedBy;
  final DateTime? decidedAt;
  final double disbursedAmount;
  final DateTime? disbursedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  /// Processing fee + GST, fixed at sanction time against the sanctioned
  /// amount and the customer's KYC address state — kept as separate line
  /// items throughout (never merged into the loan principal). `gstType` is
  /// "CGST_SGST" for an intra-state customer or "IGST" for inter-state;
  /// all these are 0/empty until the loan is sanctioned.
  final double processingFeeBase;
  final double processingFeeCgst;
  final double processingFeeSgst;
  final double processingFeeIgst;
  final double processingFeeGstTotal;
  final double processingFeeTotal;
  final String processingFeeGstType;
  final String processingFeeCustomerState;

  /// Set only for loans created via `POST /auth/loans/:id/topup` — the ID
  /// of the disbursed loan this top-up application is linked to.
  final int? parentLoanId;

  /// Deterministic mock CIBIL score (550-850), generated once per customer
  /// on their first application (`internal/loans/reference.go`
  /// `getOrCreateCibilScore`). Drives interest-rate rules and rejection
  /// decisions server-side; shown to the customer only via the sanction
  /// letter, not surfaced as dedicated UI here.
  final int cibilScore;

  /// Masked references submitted with this application — always present
  /// (exactly 2) on any full loan-detail response.
  final List<LoanReferenceInfo> references;

  const LoanApplication({
    required this.id,
    required this.category,
    required this.amountRequested,
    required this.purpose,
    required this.applicantName,
    required this.applicantPhone,
    required this.addressLine,
    required this.city,
    required this.status,
    required this.autoVerified,
    this.autoVerifiedNote,
    this.assignedEmployeeId,
    this.verificationReport,
    this.verificationFindings,
    this.verificationRisk,
    this.verificationRecommendation,
    this.decisionRemarks,
    this.decidedBy,
    this.decidedAt,
    required this.disbursedAmount,
    this.disbursedAt,
    required this.createdAt,
    required this.updatedAt,
    this.parentLoanId,
    this.cibilScore = 0,
    this.references = const [],
    this.processingFeeBase = 0,
    this.processingFeeCgst = 0,
    this.processingFeeSgst = 0,
    this.processingFeeIgst = 0,
    this.processingFeeGstTotal = 0,
    this.processingFeeTotal = 0,
    this.processingFeeGstType = '',
    this.processingFeeCustomerState = '',
  });

  factory LoanApplication.fromJson(Map<String, dynamic> json) {
    DateTime? parseDate(dynamic v) => v == null ? null : DateTime.tryParse(v as String);
    return LoanApplication(
      id: json['id'] as int,
      category: json['category'] as String? ?? '',
      amountRequested: (json['amountRequested'] as num?)?.toDouble() ?? 0,
      purpose: json['purpose'] as String? ?? '',
      applicantName: json['applicantName'] as String? ?? '',
      applicantPhone: json['applicantPhone'] as String? ?? '',
      addressLine: json['addressLine'] as String? ?? '',
      city: json['city'] as String? ?? '',
      status: json['status'] as String? ?? '',
      autoVerified: json['autoVerified'] as bool? ?? false,
      autoVerifiedNote: json['autoVerifiedNote'] as String?,
      assignedEmployeeId: json['assignedEmployeeId'] as int?,
      verificationReport: json['verificationReport'] as String?,
      verificationFindings: json['verificationFindings'] as String?,
      verificationRisk: json['verificationRisk'] as String?,
      verificationRecommendation: json['verificationRecommendation'] as String?,
      decisionRemarks: json['decisionRemarks'] as String?,
      decidedBy: json['decidedBy'] as String?,
      decidedAt: parseDate(json['decidedAt']),
      disbursedAmount: (json['disbursedAmount'] as num?)?.toDouble() ?? 0,
      disbursedAt: parseDate(json['disbursedAt']),
      createdAt: parseDate(json['createdAt']) ?? DateTime.now(),
      updatedAt: parseDate(json['updatedAt']) ?? DateTime.now(),
      parentLoanId: json['parentLoanId'] as int?,
      cibilScore: json['cibilScore'] as int? ?? 0,
      references: (json['references'] as List<dynamic>?)
              ?.map((e) => LoanReferenceInfo.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
      processingFeeBase: (json['processingFeeBase'] as num?)?.toDouble() ?? 0,
      processingFeeCgst: (json['processingFeeCgst'] as num?)?.toDouble() ?? 0,
      processingFeeSgst: (json['processingFeeSgst'] as num?)?.toDouble() ?? 0,
      processingFeeIgst: (json['processingFeeIgst'] as num?)?.toDouble() ?? 0,
      processingFeeGstTotal: (json['processingFeeGstTotal'] as num?)?.toDouble() ?? 0,
      processingFeeTotal: (json['processingFeeTotal'] as num?)?.toDouble() ?? 0,
      processingFeeGstType: json['processingFeeGstType'] as String? ?? '',
      processingFeeCustomerState: json['processingFeeCustomerState'] as String? ?? '',
    );
  }
}

/// Maps the backend's raw status strings (`internal/models/loan.go`) to
/// customer-friendly copy — never show the raw enum value in the UI.
class LoanStatusInfo {
  const LoanStatusInfo(this.label, this.description);

  final String label;
  final String description;

  static const _map = <String, LoanStatusInfo>{
    'submitted': LoanStatusInfo('Submitted', 'Your application has been received.'),
    'auto_verified': LoanStatusInfo('Under Review', 'Auto-verified — awaiting final approval.'),
    'assigned': LoanStatusInfo('Under Verification', 'Assigned to our verification team.'),
    'verified': LoanStatusInfo('Verified', 'Verification complete — awaiting sanction decision.'),
    'sanctioned': LoanStatusInfo('Sanctioned', 'Sanctioned — ready for disbursement.'),
    'rejected': LoanStatusInfo('Rejected', 'This application was not approved.'),
    'disbursed': LoanStatusInfo('Disbursed', 'Loan amount has been disbursed to your account.'),
  };

  static LoanStatusInfo forStatus(String status) =>
      _map[status] ?? LoanStatusInfo(status, 'Status: $status');
}
