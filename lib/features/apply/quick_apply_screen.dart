import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/auth_api_service.dart';
import '../../core/network/kyc_api_service.dart';
import '../../core/network/loan_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/signed_documents_store.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../core/widgets/signature_pad.dart';
import '../../core/widgets/slider_input_field.dart';
import '../../core/services/notification_service.dart';
import '../../core/widgets/success_celebration.dart';
import '../../mock/india_locations.dart';
import '../../models/loan_product.dart';
import 'widgets/apply_form_field.dart';
import 'widgets/bank_document_upload_tile.dart';
import 'widgets/document_upload_tile.dart';
import 'widgets/loan_agreement_pdf.dart';

enum _EmploymentType { salaried, selfEmployed, businessOwner }

/// Full loan application wizard, usable for any [LoanProduct] (or the
/// generic pre-approved offer when none is passed): Details → Address →
/// KYC Documents → Terms & Conditions → Submit → Success.
///
/// RBI-inspired touches (Digital Lending Guidelines 2022 + the Oct 2024
/// Key Facts Statement circular): a KFS-style disclosure before consent,
/// and three separate, itemized consent checkboxes rather than one
/// bundled "I agree to everything" box — real digital lenders are
/// required to capture consent for the loan terms, the credit bureau
/// pull, and marketing/communication preferences independently, not as
/// a single checkbox. Frontend-only — nothing is submitted anywhere.
class QuickApplyScreen extends StatefulWidget {
  const QuickApplyScreen({super.key, this.product});

  final LoanProduct? product;

  @override
  State<QuickApplyScreen> createState() => _QuickApplyScreenState();
}

class _QuickApplyScreenState extends State<QuickApplyScreen> {
  final PageController _pageController = PageController();
  int _step = 0;

  /// Instant Loan used to skip video KYC scheduling entirely (it
  /// auto-approved server-side with no employee involvement at all) — now
  /// every category, Instant Loan included, goes through real employee
  /// verification and gets the same Verification Scheduling step.
  bool get _skipVideoKyc => false;

  bool get _isBusinessLoan => widget.product?.category == LoanCategory.business;
  bool get _showsProcessingNotice =>
      widget.product?.category == LoanCategory.housing || widget.product?.category == LoanCategory.business;

  int get _stepCount => 6;
  int get _kycStep => 2;
  int get _bankStep => 3;
  int get _specialStep => 4; // Verification scheduling, or Instant Loan consent

  List<String> get _stepTitles => [
        'Details',
        'Address',
        'KYC Documents',
        'Bank Details',
        _skipVideoKyc ? 'Loan Consent' : 'Verification',
        'Terms & Conditions',
      ];

  double get _interestRate => widget.product?.interestRateFrom ?? 10.5;
  double get _aprRate => widget.product?.aprFrom ?? 11.4;
  String get _productName => widget.product?.name ?? 'Personal Loan';
  String get _processingFee => widget.product?.processingFee ?? 'Up to 2.5% + GST';

  // Step 0: Details
  final _detailsFormKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _dobController = TextEditingController();
  DateTime? _dobValue;
  final _panController = TextEditingController();
  final _emailController = TextEditingController();
  _EmploymentType _employmentType = _EmploymentType.salaried;
  final _monthlyIncomeController = TextEditingController();
  double _loanAmount = 500000;
  double _tenureMonths = 36;

  // Step 1: Address
  final _addressFormKey = GlobalKey<FormState>();
  final _addressLineController = TextEditingController();
  final _pinCodeController = TextEditingController();
  String? _selectedState;
  String? _selectedCity;

  // Step 2: KYC Documents
  bool _aadhaarVerified = false;
  bool _panVerified = false;
  bool _photoVerified = false;
  bool _electricityBillVerified = false;
  bool _bankStatementVerified = false;
  bool _isRented = false;
  bool _rentalAgreementVerified = false;
  final _aadhaarNumberController = TextEditingController();
  bool _aadhaarConsent = false;
  File? _photoFile;

  // Business Loan only — additional required documents.
  bool _gstCertificateVerified = false;
  bool _businessPremisesVerified = false;

  bool get _allDocumentsVerified =>
      _aadhaarVerified &&
      _panVerified &&
      _photoVerified &&
      _electricityBillVerified &&
      _bankStatementVerified &&
      (!_isRented || _rentalAgreementVerified) &&
      _aadhaarNumberController.text.trim().length == 12 &&
      _aadhaarConsent &&
      (!_isBusinessLoan || (_gstCertificateVerified && _businessPremisesVerified));

  // Step 3: Bank Details
  final _bankFormKey = GlobalKey<FormState>();
  final _accountHolderController = TextEditingController();
  final _accountNumberController = TextEditingController();
  final _confirmAccountNumberController = TextEditingController();
  final _ifscController = TextEditingController();
  final _bankNameController = TextEditingController();
  String _accountType = 'Savings';

  // Step 5: Verification Scheduling (non-Instant-Loan products)
  String? _timingPreference;
  DateTime? _visitDate;
  String? _visitTimeSlot;

  bool get _requiresPropertyVisit {
    final category = widget.product?.category;
    return category == LoanCategory.housing || category == LoanCategory.loanAgainstProperty || category == LoanCategory.warehouseFinance;
  }

  // Step 5: Instant Loan consent (Instant Loan only)
  bool _instantConsentAgreed = false;

  // Step 6: Terms & Conditions + e-Sign
  Uint8List? _signatureBytes;
  bool _isSubmitting = false;
  bool _submitted = false;
  String _applicationId = 'NBFC-APP-${DateTime.now().millisecondsSinceEpoch % 1000000}';

  double get _emi {
    final rate = _interestRate / 12 / 100;
    final n = _tenureMonths;
    double compounded = 1;
    for (int i = 0; i < n; i++) {
      compounded *= 1 + rate;
    }
    return _loanAmount * rate * compounded / (compounded - 1);
  }

  @override
  void initState() {
    super.initState();
    _prefillFromProfile();
  }

  /// Best-effort prefill of name/email from the customer's real saved
  /// profile — this used to always show MockData's hardcoded "Rajesh
  /// Verma" regardless of who was actually applying, meaning a
  /// distracted customer could submit an application under someone
  /// else's name if they didn't notice and edit it themselves. A failure
  /// here (offline, profile not created yet) just leaves the fields
  /// blank for the customer to fill in, same as before this existed.
  Future<void> _prefillFromProfile() async {
    final token = AuthProvider.instance.token;
    if (token == null) return;
    try {
      final res = await AuthApiService.getProfile(token);
      final user = res['user'] as Map<String, dynamic>?;
      final first = (user?['first_name'] as String?)?.trim() ?? '';
      final last = (user?['last_name'] as String?)?.trim() ?? '';
      final fullName = [first, last].where((s) => s.isNotEmpty).join(' ');
      final email = (user?['email'] as String?)?.trim() ?? '';
      if (!mounted) return;
      if (fullName.isNotEmpty) {
        _nameController.text = fullName;
        _accountHolderController.text = fullName;
      }
      if (email.isNotEmpty) _emailController.text = email;
    } on ApiException {
      // Non-fatal.
    } catch (_) {
      // Non-fatal.
    }
  }

  @override
  void dispose() {
    _pageController.dispose();
    _nameController.dispose();
    _dobController.dispose();
    _panController.dispose();
    _emailController.dispose();
    _monthlyIncomeController.dispose();
    _addressLineController.dispose();
    _pinCodeController.dispose();
    _accountHolderController.dispose();
    _accountNumberController.dispose();
    _confirmAccountNumberController.dispose();
    _ifscController.dispose();
    _bankNameController.dispose();
    _aadhaarNumberController.dispose();
    super.dispose();
  }

  bool get _canProceedFromCurrentStep {
    if (_step == 0) return _detailsFormKey.currentState?.validate() ?? false;
    if (_step == 1) return _addressFormKey.currentState?.validate() ?? false;
    if (_step == _kycStep) return _allDocumentsVerified;
    if (_step == _bankStep) return _bankFormKey.currentState?.validate() ?? false;
    if (_step == _specialStep) {
      if (_skipVideoKyc) return _instantConsentAgreed;
      return _timingPreference != null && (!_requiresPropertyVisit || (_visitDate != null && _visitTimeSlot != null));
    }
    return true;
  }

  void _next() {
    if (!_canProceedFromCurrentStep) {
      if (_step == _kycStep) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              _aadhaarNumberController.text.trim().length != 12
                  ? 'Please enter a valid 12-digit Aadhaar number.'
                  : !_aadhaarConsent
                      ? 'Please provide Aadhaar verification consent to continue.'
                      : _isBusinessLoan && (!_gstCertificateVerified || !_businessPremisesVerified)
                          ? 'Please upload the GST Certificate and Proof of Business Premises to continue.'
                          : 'Please upload and verify all KYC documents to continue.',
            ),
          ),
        );
      } else if (_step == _specialStep) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              _skipVideoKyc
                  ? 'Please read and agree to the loan terms to continue.'
                  : _timingPreference == null
                      ? 'Please select your preferred timing for the video KYC call.'
                      : 'Please select a date and time for the property visit.',
            ),
          ),
        );
      }
      return;
    }
    if (_step < _stepCount - 1) {
      setState(() => _step++);
      _pageController.animateToPage(_step, duration: const Duration(milliseconds: 280), curve: Curves.easeOutCubic);
    }
  }

  void _back() {
    if (_step > 0) {
      setState(() => _step--);
      _pageController.animateToPage(_step, duration: const Duration(milliseconds: 280), curve: Curves.easeOutCubic);
    } else {
      context.pop();
    }
  }

  bool get _canSubmit => _signatureBytes != null;

  String get _employmentTypeLabel => switch (_employmentType) {
        _EmploymentType.salaried => 'Salaried',
        _EmploymentType.selfEmployed => 'Self-Employed',
        _EmploymentType.businessOwner => 'Business Owner',
      };

  /// Runs a KYC sub-step, treating a 409 ("already exists") as success —
  /// several of the backend's create-* endpoints are write-once and
  /// reject a second submission outright rather than upserting, which
  /// would otherwise block every retry of a partially-submitted
  /// application (e.g. one that failed on a later step).
  Future<void> _submitOrSkipIfExists(Future<void> Function() call) async {
    try {
      await call();
    } on ApiException catch (e) {
      if (e.statusCode == 409) return;
      rethrow;
    }
  }

  Future<void> _submitToBackend() async {
    final token = AuthProvider.instance.token;
    if (token == null) return; // no session — nothing to submit against

    final nameParts = _nameController.text.trim().split(RegExp(r'\s+'));
    final firstName = nameParts.isNotEmpty ? nameParts.first : _nameController.text.trim();
    final lastName = nameParts.length > 1 ? nameParts.sublist(1).join(' ') : firstName;

    await _submitOrSkipIfExists(() => KycApiService.createProfile(
          token,
          firstName: firstName,
          lastName: lastName,
          email: _emailController.text.trim(),
          dateOfBirth: _dobValue != null ? DateFormat('yyyy-MM-dd').format(_dobValue!) : '',
          gender: 'Prefer not to say',
          maritalStatus: 'Prefer not to say',
          occupation: _employmentTypeLabel,
        ));

    await KycApiService.createAddress(
      token,
      currentAddress: _addressLineController.text.trim(),
      currentState: _selectedState ?? '',
      currentDistrict: _selectedCity ?? '',
      currentCity: _selectedCity ?? '',
      currentPincode: _pinCodeController.text.trim(),
    );

    await _submitOrSkipIfExists(() => KycApiService.createPan(
          token,
          panNumber: _panController.text.trim(),
          nameOnPan: _nameController.text.trim(),
        ));

    await _submitOrSkipIfExists(() => KycApiService.createAadhaar(
          token,
          aadhaarNumber: _aadhaarNumberController.text.trim(),
          nameOnAadhaar: _nameController.text.trim(),
          consent: _aadhaarConsent,
        ));

    final photo = _photoFile;
    if (photo != null) {
      final base64Image = base64Encode(await photo.readAsBytes());
      await _submitOrSkipIfExists(() => KycApiService.createSelfie(token, imageBase64: base64Image));
    }

    await KycApiService.createEmployment(
      token,
      employmentType: _employmentTypeLabel,
      companyName: 'Not specified',
      occupation: _employmentTypeLabel,
      monthlyIncome: double.tryParse(_monthlyIncomeController.text.trim()) ?? 0,
    );

    // Category is the loan product's display name, which is exactly what
    // the backend's auto-approve map keys on ("Instant Loan" / "Personal
    // Loan" — see internal/loans/service.go).
    final loan = await LoanApiService.apply(
      token,
      category: _productName,
      amountRequested: _loanAmount,
      tenureMonths: _tenureMonths.round(),
      monthlyIncome: double.tryParse(_monthlyIncomeController.text.trim()) ?? 0,
      purpose: widget.product?.shortDescription ?? 'General purpose',
      applicantName: _nameController.text.trim(),
      applicantPhone: AuthProvider.instance.pendingPhoneNumber ?? '',
      addressLine: _addressLineController.text.trim(),
      city: _selectedCity ?? '',
      timingPreference: _timingPreference,
      visitDate: _visitDate,
      visitTimeSlot: _visitTimeSlot,
    );
    _applicationId = 'NBFC-APP-${loan.id}';
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() => _isSubmitting = true);
    try {
      await _submitToBackend();
      await _saveSignedAgreement();
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _isSubmitting = false);
      // A 422 here is the eligibility engine (internal/loans/eligibility.go)
      // rejecting the amount/tenure/income combination — e.message is
      // already the specific, actionable reason ("reduce the amount to
      // about ₹X…"), so it's shown as-is rather than wrapped in the
      // generic "Could not save your KYC details" framing that fits
      // every earlier step but not this one.
      final message = e.statusCode == 422 ? e.message : 'Could not save your details: ${e.message}';
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(message)),
      );
      return;
    }
    if (!mounted) return;
    setState(() {
      _isSubmitting = false;
      _submitted = true;
    });
    NotificationService.instance.show(
      title: 'Application Submitted',
      body: _skipVideoKyc
          ? 'Your ${widget.product?.name ?? 'loan'} application was received and is being processed instantly.'
          : 'Your ${widget.product?.name ?? 'loan'} application was received. Our employee will contact you for video KYC — keep your documentation ready.',
    );
  }

  Future<void> _saveSignedAgreement() async {
    final signature = _signatureBytes;
    if (signature == null || signature.isEmpty) return;
    try {
      final pdfBytes = await LoanAgreementPdf.build(
        applicantName: _nameController.text.trim(),
        productName: _productName,
        loanAmount: _loanAmount,
        tenureMonths: _tenureMonths,
        interestRate: _interestRate,
        aprRate: _aprRate,
        processingFee: _processingFee,
        emi: _emi,
        applicationId: _applicationId,
        signatureImage: signature,
      );
      await SignedDocumentsStore.save('Loan Agreement · $_productName', pdfBytes);
    } catch (_) {
      // Non-fatal — the application itself was already submitted to the
      // backend; a failure here shouldn't block the success screen.
    }
  }

  Future<void> _pickDob() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: DateTime(1995, 1, 1),
      firstDate: DateTime(1940),
      lastDate: DateTime.now().subtract(const Duration(days: 365 * 18)),
    );
    if (picked != null) {
      _dobValue = picked;
      _dobController.text = DateFormat('d MMM yyyy').format(picked);
    }
  }

  Future<void> _pickVisitDate() async {
    final now = DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: now.add(const Duration(days: 2)),
      firstDate: now.add(const Duration(days: 1)),
      lastDate: now.add(const Duration(days: 30)),
    );
    if (picked != null) {
      setState(() => _visitDate = picked);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_submitted) {
      return _SuccessView(
        applicationId: _applicationId,
        productName: _productName,
        loanAmount: _loanAmount,
        tenureMonths: _tenureMonths,
        emi: _emi,
        timingPreference: _timingPreference,
        requiresPropertyVisit: _requiresPropertyVisit,
        visitDate: _visitDate,
        visitTimeSlot: _visitTimeSlot,
        skipVideoKyc: _skipVideoKyc,
        showsProcessingNotice: _showsProcessingNotice,
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text('Apply · ${_stepTitles[_step]}'),
        leading: IconButton(icon: const Icon(Icons.arrow_back_rounded), onPressed: _back),
      ),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 4),
              child: Row(
                children: [
                  for (int i = 0; i < _stepCount; i++) ...[
                    Expanded(
                      child: AnimatedContainer(
                        duration: const Duration(milliseconds: 200),
                        height: 4,
                        decoration: BoxDecoration(
                          color: i <= _step ? AppColors.primary : AppColors.borderLight,
                          borderRadius: BorderRadius.circular(AppRadius.pill),
                        ),
                      ),
                    ),
                    if (i != _stepCount - 1) const SizedBox(width: 4),
                  ],
                ],
              ),
            ),
            Expanded(
              child: PageView(
                controller: _pageController,
                physics: const NeverScrollableScrollPhysics(),
                children: [
                  _DetailsStep(
                    formKey: _detailsFormKey,
                    productName: _productName,
                    nameController: _nameController,
                    dobController: _dobController,
                    panController: _panController,
                    emailController: _emailController,
                    monthlyIncomeController: _monthlyIncomeController,
                    employmentType: _employmentType,
                    onEmploymentTypeChanged: (v) => setState(() => _employmentType = v ?? _employmentType),
                    onPickDob: _pickDob,
                    loanAmount: _loanAmount,
                    onLoanAmountChanged: (v) => setState(() => _loanAmount = v),
                    tenureMonths: _tenureMonths,
                    onTenureChanged: (v) => setState(() => _tenureMonths = v),
                    interestRate: _interestRate,
                  ),
                  _AddressStep(
                    formKey: _addressFormKey,
                    addressLineController: _addressLineController,
                    pinCodeController: _pinCodeController,
                    selectedState: _selectedState,
                    selectedCity: _selectedCity,
                    onStateChanged: (v) => setState(() {
                      _selectedState = v;
                      _selectedCity = null;
                    }),
                    onCityChanged: (v) => setState(() => _selectedCity = v),
                  ),
                  _KycDocumentsStep(
                    onAadhaarChanged: (v) => setState(() => _aadhaarVerified = v),
                    onPanChanged: (v) => setState(() => _panVerified = v),
                    onPhotoChanged: (v) => setState(() => _photoVerified = v),
                    onPhotoCaptured: (f) => _photoFile = f,
                    aadhaarNumberController: _aadhaarNumberController,
                    aadhaarConsent: _aadhaarConsent,
                    onAadhaarConsentChanged: (v) => setState(() => _aadhaarConsent = v ?? false),
                    onElectricityBillChanged: (v) => setState(() => _electricityBillVerified = v),
                    onBankStatementChanged: (v) => setState(() => _bankStatementVerified = v),
                    isRented: _isRented,
                    onIsRentedChanged: (v) => setState(() => _isRented = v ?? false),
                    onRentalAgreementChanged: (v) => setState(() => _rentalAgreementVerified = v),
                    isBusinessLoan: _isBusinessLoan,
                    onGstCertificateChanged: (v) => setState(() => _gstCertificateVerified = v),
                    onBusinessPremisesChanged: (v) => setState(() => _businessPremisesVerified = v),
                  ),
                  _BankDetailsStep(
                    formKey: _bankFormKey,
                    accountHolderController: _accountHolderController,
                    accountNumberController: _accountNumberController,
                    confirmAccountNumberController: _confirmAccountNumberController,
                    ifscController: _ifscController,
                    bankNameController: _bankNameController,
                    accountType: _accountType,
                    onAccountTypeChanged: (v) => setState(() => _accountType = v ?? _accountType),
                  ),
                  _skipVideoKyc
                      ? _InstantLoanConsentStep(
                          agreed: _instantConsentAgreed,
                          onAgreedChanged: (v) => setState(() => _instantConsentAgreed = v ?? false),
                        )
                      : _VerificationSchedulingStep(
                          timingPreference: _timingPreference,
                          onTimingPreferenceChanged: (v) => setState(() => _timingPreference = v),
                          requiresPropertyVisit: _requiresPropertyVisit,
                          visitDate: _visitDate,
                          onPickVisitDate: _pickVisitDate,
                          visitTimeSlot: _visitTimeSlot,
                          onVisitTimeSlotChanged: (v) => setState(() => _visitTimeSlot = v),
                        ),
                  _TermsStep(
                    productName: _productName,
                    loanAmount: _loanAmount,
                    tenureMonths: _tenureMonths,
                    interestRate: _interestRate,
                    aprRate: _aprRate,
                    processingFee: _processingFee,
                    emi: _emi,
                    signatureBytes: _signatureBytes,
                    onSigned: (bytes) => setState(() => _signatureBytes = bytes),
                    onClearSignature: () => setState(() => _signatureBytes = null),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 8, 20, 16),
              child: _step == _stepCount - 1
                  ? ElevatedButton(
                      onPressed: _canSubmit && !_isSubmitting ? _submit : null,
                      style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                      child: _isSubmitting
                          ? const SizedBox(
                              width: 22,
                              height: 22,
                              child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                            )
                          : const Text('Submit Application'),
                    )
                  : ElevatedButton(
                      onPressed: _next,
                      style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                      child: const Text('Continue'),
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

class UpperCaseTextFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(TextEditingValue oldValue, TextEditingValue newValue) {
    return newValue.copyWith(text: newValue.text.toUpperCase(), selection: newValue.selection);
  }
}

class _DetailsStep extends StatelessWidget {
  const _DetailsStep({
    required this.formKey,
    required this.productName,
    required this.nameController,
    required this.dobController,
    required this.panController,
    required this.emailController,
    required this.monthlyIncomeController,
    required this.employmentType,
    required this.onEmploymentTypeChanged,
    required this.onPickDob,
    required this.loanAmount,
    required this.onLoanAmountChanged,
    required this.tenureMonths,
    required this.onTenureChanged,
    required this.interestRate,
  });

  final GlobalKey<FormState> formKey;
  final String productName;
  final TextEditingController nameController;
  final TextEditingController dobController;
  final TextEditingController panController;
  final TextEditingController emailController;
  final TextEditingController monthlyIncomeController;
  final _EmploymentType employmentType;
  final ValueChanged<_EmploymentType?> onEmploymentTypeChanged;
  final VoidCallback onPickDob;
  final double loanAmount;
  final ValueChanged<double> onLoanAmountChanged;
  final double tenureMonths;
  final ValueChanged<double> onTenureChanged;
  final double interestRate;

  String _label(_EmploymentType type) => switch (type) {
        _EmploymentType.salaried => 'Salaried',
        _EmploymentType.selfEmployed => 'Self-Employed',
        _EmploymentType.businessOwner => 'Business Owner',
      };

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          Text('Applying for', style: theme.textTheme.bodySmall),
          Text(productName, style: theme.textTheme.titleLarge, maxLines: 1, overflow: TextOverflow.ellipsis),
          const SizedBox(height: 16),
          PremiumCard(
            child: Column(
              children: [
                SliderInputField(
                  label: 'Loan Amount Required',
                  value: loanAmount,
                  min: 50000,
                  max: 5000000,
                  divisions: 99,
                  fieldWidth: 130,
                  onChanged: onLoanAmountChanged,
                ),
                SliderInputField(
                  label: 'Tenure',
                  value: tenureMonths,
                  min: 12,
                  max: 60,
                  divisions: 48,
                  suffix: ' mo',
                  onChanged: onTenureChanged,
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          ValueListenableBuilder<TextEditingValue>(
            valueListenable: monthlyIncomeController,
            builder: (context, incomeValue, _) {
              final income = double.tryParse(incomeValue.text.trim()) ?? 0;
              if (income <= 0) return const SizedBox.shrink();

              // Mirrors the backend's eligibility engine
              // (internal/loans/eligibility.go) for a live estimate —
              // the server has the final say (real CIBIL-based rate,
              // enforced at submit), this is just an early, honest
              // heads-up instead of letting the customer fill out the
              // entire rest of the form before finding out the amount
              // won't be approved.
              final r = interestRate / 12 / 100;
              final n = tenureMonths;
              double compounded = 1;
              for (int i = 0; i < n; i++) {
                compounded *= 1 + r;
              }
              final emi = loanAmount * r * compounded / (compounded - 1);
              final foirPercent = (emi / income) * 100;
              final overLimit = foirPercent > 50;

              return Padding(
                padding: const EdgeInsets.only(top: 4, bottom: 4),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                  decoration: BoxDecoration(
                    color: (overLimit ? AppColors.error : AppColors.success).withValues(alpha: 0.08),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                    border: Border.all(color: (overLimit ? AppColors.error : AppColors.success).withValues(alpha: 0.25)),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(
                        overLimit ? Icons.warning_amber_rounded : Icons.check_circle_outline,
                        size: 18,
                        color: overLimit ? AppColors.error : AppColors.success,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          overLimit
                              ? 'Estimated EMI ₹${emi.toStringAsFixed(0)}/mo is ${foirPercent.toStringAsFixed(0)}% of your income — reduce the amount or increase the tenure to stay under 50%.'
                              : 'Estimated EMI ₹${emi.toStringAsFixed(0)}/mo (${foirPercent.toStringAsFixed(0)}% of income) — within the usual eligibility range.',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: overLimit ? AppColors.error : AppColors.success,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
          const SizedBox(height: 8),
          ApplyFormField(
            label: 'Full Name',
            controller: nameController,
            hint: 'Enter your full name',
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your full name' : null,
          ),
          ApplyFormField(
            label: 'Date of Birth',
            controller: dobController,
            readOnly: true,
            onTap: onPickDob,
            hint: 'Select date',
            validator: (v) => (v == null || v.isEmpty) ? 'Select your date of birth' : null,
          ),
          ApplyFormField(
            label: 'PAN Number',
            controller: panController,
            hint: 'ABCDE1234F',
            maxLength: 10,
            inputFormatters: [UpperCaseTextFormatter()],
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Enter your PAN number';
              if (!RegExp(r'^[A-Z]{5}[0-9]{4}[A-Z]$').hasMatch(v.trim())) return 'Enter a valid PAN (e.g. ABCDE1234F)';
              return null;
            },
          ),
          ApplyFormField(
            label: 'Email Address',
            controller: emailController,
            keyboardType: TextInputType.emailAddress,
            hint: 'you@example.com',
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Enter your email';
              if (!v.contains('@') || !v.contains('.')) return 'Enter a valid email';
              return null;
            },
          ),
          ApplyFormDropdown<_EmploymentType>(
            label: 'Employment Type',
            value: employmentType,
            items: _EmploymentType.values,
            labelBuilder: _label,
            onChanged: onEmploymentTypeChanged,
          ),
          ApplyFormField(
            label: 'Monthly Net Income (₹)',
            controller: monthlyIncomeController,
            keyboardType: TextInputType.number,
            hint: 'Enter your monthly income',
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Enter your monthly income';
              if ((int.tryParse(v) ?? 0) < 15000) return 'Minimum income requirement is ₹15,000';
              return null;
            },
          ),
        ],
      ),
    );
  }
}

class _AddressStep extends StatelessWidget {
  const _AddressStep({
    required this.formKey,
    required this.addressLineController,
    required this.pinCodeController,
    required this.selectedState,
    required this.selectedCity,
    required this.onStateChanged,
    required this.onCityChanged,
  });

  final GlobalKey<FormState> formKey;
  final TextEditingController addressLineController;
  final TextEditingController pinCodeController;
  final String? selectedState;
  final String? selectedCity;
  final ValueChanged<String?> onStateChanged;
  final ValueChanged<String?> onCityChanged;

  @override
  Widget build(BuildContext context) {
    final cities = IndiaLocations.citiesFor(selectedState);
    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          ApplyFormField(
            label: 'Address',
            controller: addressLineController,
            hint: 'House/flat no., street, locality',
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your address' : null,
          ),
          _StateDropdown(value: selectedState, onChanged: onStateChanged),
          if (selectedState == null)
            Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: Text('Select a state to choose your city', style: Theme.of(context).textTheme.labelSmall),
            )
          else
            _CityDropdown(value: selectedCity, cities: cities, onChanged: onCityChanged),
          ApplyFormField(
            label: 'PIN Code',
            controller: pinCodeController,
            keyboardType: TextInputType.number,
            maxLength: 6,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            validator: (v) => (v == null || v.trim().length != 6) ? 'Enter a valid 6-digit PIN code' : null,
          ),
        ],
      ),
    );
  }
}

class _StateDropdown extends StatelessWidget {
  const _StateDropdown({required this.value, required this.onChanged});

  final String? value;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('State', style: Theme.of(context).textTheme.titleSmall),
        const SizedBox(height: 8),
        DropdownButtonFormField<String>(
          value: value,
          hint: const Text('Select state'),
          items: [for (final s in IndiaLocations.states) DropdownMenuItem(value: s, child: Text(s))],
          onChanged: onChanged,
          validator: (v) => v == null ? 'Select your state' : null,
          decoration: InputDecoration(
            filled: true,
            fillColor: Theme.of(context).brightness == Brightness.dark ? AppColors.surfaceDark : AppColors.surfaceLight,
            contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
            enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
            focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.primary, width: 1.6)),
          ),
        ),
        const SizedBox(height: 16),
      ],
    );
  }
}

class _CityDropdown extends StatelessWidget {
  const _CityDropdown({required this.value, required this.cities, required this.onChanged});

  final String? value;
  final List<String> cities;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('City', style: Theme.of(context).textTheme.titleSmall),
        const SizedBox(height: 8),
        DropdownButtonFormField<String>(
          value: value,
          hint: const Text('Select city'),
          items: [for (final c in cities) DropdownMenuItem(value: c, child: Text(c))],
          onChanged: onChanged,
          validator: (v) => v == null ? 'Select your city' : null,
          decoration: InputDecoration(
            filled: true,
            fillColor: Theme.of(context).brightness == Brightness.dark ? AppColors.surfaceDark : AppColors.surfaceLight,
            contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
            enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
            focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.primary, width: 1.6)),
          ),
        ),
        const SizedBox(height: 16),
      ],
    );
  }
}

class _KycDocumentsStep extends StatelessWidget {
  const _KycDocumentsStep({
    required this.onAadhaarChanged,
    required this.onPanChanged,
    required this.onPhotoChanged,
    required this.onPhotoCaptured,
    required this.aadhaarNumberController,
    required this.aadhaarConsent,
    required this.onAadhaarConsentChanged,
    required this.onElectricityBillChanged,
    required this.onBankStatementChanged,
    required this.isRented,
    required this.onIsRentedChanged,
    required this.onRentalAgreementChanged,
    required this.isBusinessLoan,
    required this.onGstCertificateChanged,
    required this.onBusinessPremisesChanged,
  });

  final ValueChanged<bool> onAadhaarChanged;
  final ValueChanged<bool> onPanChanged;
  final ValueChanged<bool> onPhotoChanged;
  final ValueChanged<File> onPhotoCaptured;
  final TextEditingController aadhaarNumberController;
  final bool aadhaarConsent;
  final ValueChanged<bool?> onAadhaarConsentChanged;
  final ValueChanged<bool> onElectricityBillChanged;
  final ValueChanged<bool> onBankStatementChanged;
  final bool isRented;
  final ValueChanged<bool?> onIsRentedChanged;
  final ValueChanged<bool> onRentalAgreementChanged;

  /// Business Loan only — two extra required documents.
  final bool isBusinessLoan;
  final ValueChanged<bool> onGstCertificateChanged;
  final ValueChanged<bool> onBusinessPremisesChanged;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Text('Upload KYC Documents', style: theme.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text(
          'Take a photo or choose from your gallery, then crop it. Each document is verified automatically.',
          style: theme.textTheme.bodySmall,
        ),
        const SizedBox(height: 16),
        ApplyFormField(
          label: 'Aadhaar Number',
          controller: aadhaarNumberController,
          hint: '12-digit Aadhaar number',
          maxLength: 12,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          validator: (v) => (v == null || v.trim().length != 12) ? 'Enter a valid 12-digit Aadhaar number' : null,
        ),
        DocumentUploadTile(label: 'Aadhaar Card', icon: Icons.fingerprint_rounded, onStatusChanged: onAadhaarChanged),
        const SizedBox(height: 10),
        DocumentUploadTile(label: 'PAN Card', icon: Icons.badge_rounded, onStatusChanged: onPanChanged),
        const SizedBox(height: 10),
        DocumentUploadTile(label: 'Photograph', icon: Icons.face_rounded, onStatusChanged: onPhotoChanged, onFileCaptured: onPhotoCaptured),
        const SizedBox(height: 4),
        CheckboxListTile(
          value: aadhaarConsent,
          onChanged: onAadhaarConsentChanged,
          controlAffinity: ListTileControlAffinity.leading,
          contentPadding: EdgeInsets.zero,
          activeColor: AppColors.primary,
          title: Text(
            'I consent to NBFC Premium verifying my Aadhaar details for this application.',
            style: theme.textTheme.bodySmall,
          ),
        ),
        const SizedBox(height: 20),
        Text('Address & Income Proof', style: theme.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text(
          'Required to verify your current address and income.',
          style: theme.textTheme.bodySmall,
        ),
        const SizedBox(height: 12),
        DocumentUploadTile(
          label: 'Electricity Bill (Address Proof)',
          icon: Icons.bolt_rounded,
          onStatusChanged: onElectricityBillChanged,
        ),
        const SizedBox(height: 10),
        DocumentUploadTile(
          label: 'Bank Statement (Last 6 Months)',
          icon: Icons.receipt_long_rounded,
          onStatusChanged: onBankStatementChanged,
        ),
        const SizedBox(height: 4),
        CheckboxListTile(
          value: isRented,
          onChanged: onIsRentedChanged,
          controlAffinity: ListTileControlAffinity.leading,
          contentPadding: EdgeInsets.zero,
          activeColor: AppColors.primary,
          title: Text('I currently live in a rented house', style: theme.textTheme.bodySmall),
        ),
        if (isRented) ...[
          const SizedBox(height: 4),
          DocumentUploadTile(
            label: 'Rental Agreement',
            icon: Icons.home_work_rounded,
            onStatusChanged: onRentalAgreementChanged,
          ),
        ],
        if (isBusinessLoan) ...[
          const SizedBox(height: 20),
          Text('Business Documents', style: theme.textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(
            'Additional documents required for Business Loan applications.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 12),
          DocumentUploadTile(
            label: 'GST Certificate',
            icon: Icons.receipt_rounded,
            onStatusChanged: onGstCertificateChanged,
          ),
          const SizedBox(height: 10),
          DocumentUploadTile(
            label: 'Proof of Business Premises',
            icon: Icons.storefront_rounded,
            onStatusChanged: onBusinessPremisesChanged,
          ),
        ],
        const SizedBox(height: 12),
        PremiumCard(
          child: Row(
            children: [
              const Icon(Icons.shield_rounded, color: AppColors.primary),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  'Your documents are used only to verify your identity for this application and are not shared with third parties.',
                  style: theme.textTheme.bodySmall,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _BankDetailsStep extends StatelessWidget {
  const _BankDetailsStep({
    required this.formKey,
    required this.accountHolderController,
    required this.accountNumberController,
    required this.confirmAccountNumberController,
    required this.ifscController,
    required this.bankNameController,
    required this.accountType,
    required this.onAccountTypeChanged,
  });

  final GlobalKey<FormState> formKey;
  final TextEditingController accountHolderController;
  final TextEditingController accountNumberController;
  final TextEditingController confirmAccountNumberController;
  final TextEditingController ifscController;
  final TextEditingController bankNameController;
  final String accountType;
  final ValueChanged<String?> onAccountTypeChanged;

  static const _accountTypes = ['Savings', 'Current'];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          PremiumCard(
            child: Row(
              children: [
                const Icon(Icons.account_balance_rounded, color: AppColors.primary),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    'Your loan amount will be disbursed to this bank account. Make sure the details are accurate.',
                    style: theme.textTheme.bodySmall,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
          ApplyFormField(
            label: 'Account Holder Name',
            controller: accountHolderController,
            hint: 'As per bank records',
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter the account holder name' : null,
          ),
          ApplyFormField(
            label: 'Bank Name',
            controller: bankNameController,
            hint: 'e.g. State Bank of India',
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your bank name' : null,
          ),
          ApplyFormField(
            label: 'Account Number',
            controller: accountNumberController,
            keyboardType: TextInputType.number,
            maxLength: 18,
            hint: 'Enter your account number',
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Enter your account number';
              if (v.trim().length < 9) return 'Enter a valid account number';
              return null;
            },
          ),
          ApplyFormField(
            label: 'Confirm Account Number',
            controller: confirmAccountNumberController,
            keyboardType: TextInputType.number,
            maxLength: 18,
            hint: 'Re-enter your account number',
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Re-enter your account number';
              if (v.trim() != accountNumberController.text.trim()) return 'Account numbers do not match';
              return null;
            },
          ),
          ApplyFormField(
            label: 'IFSC Code',
            controller: ifscController,
            hint: 'ABCD0123456',
            maxLength: 11,
            inputFormatters: [UpperCaseTextFormatter()],
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Enter your bank\'s IFSC code';
              if (!RegExp(r'^[A-Z]{4}0[A-Z0-9]{6}$').hasMatch(v.trim())) return 'Enter a valid IFSC code (e.g. ABCD0123456)';
              return null;
            },
          ),
          ApplyFormDropdown<String>(
            label: 'Account Type',
            value: accountType,
            items: _accountTypes,
            labelBuilder: (v) => v,
            onChanged: onAccountTypeChanged,
          ),
          const SizedBox(height: 20),
          Text('Supporting Documents', style: theme.textTheme.titleSmall),
          const SizedBox(height: 4),
          Text(
            'Optional, but speeds up verification — a chequebook or passbook photo/PDF confirming your account details.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 12),
          const BankDocumentUploadTile(label: 'Chequebook', icon: Icons.receipt_long_rounded, docType: 'chequebook'),
          const SizedBox(height: 12),
          const BankDocumentUploadTile(label: 'Passbook', icon: Icons.menu_book_rounded, docType: 'passbook'),
        ],
      ),
    );
  }
}

class _VerificationSchedulingStep extends StatelessWidget {
  const _VerificationSchedulingStep({
    required this.timingPreference,
    required this.onTimingPreferenceChanged,
    required this.requiresPropertyVisit,
    required this.visitDate,
    required this.onPickVisitDate,
    required this.visitTimeSlot,
    required this.onVisitTimeSlotChanged,
  });

  final String? timingPreference;
  final ValueChanged<String> onTimingPreferenceChanged;
  final bool requiresPropertyVisit;
  final DateTime? visitDate;
  final VoidCallback onPickVisitDate;
  final String? visitTimeSlot;
  final ValueChanged<String> onVisitTimeSlotChanged;

  static const _timeSlots = ['9 AM – 12 PM', '12 PM – 3 PM', '3 PM – 6 PM', '6 PM – 8 PM'];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        PremiumCard(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                ),
                child: const Icon(Icons.video_call_rounded, color: AppColors.primary),
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Video KYC Verification', style: theme.textTheme.titleSmall),
                    const SizedBox(height: 4),
                    Text(
                      'Our team will call you for a quick video KYC. Please keep your original Aadhaar, PAN, and address proof ready.',
                      style: theme.textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),
        Text('Preferred Call Timing', style: theme.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text('When should our team call you for the video KYC?', style: theme.textTheme.bodySmall),
        const SizedBox(height: 12),
        Wrap(
          spacing: 10,
          runSpacing: 10,
          children: [
            for (final slot in _timeSlots)
              ChoiceChip(
                label: Text(slot),
                selected: timingPreference == slot,
                onSelected: (_) => onTimingPreferenceChanged(slot),
                selectedColor: AppColors.primary.withValues(alpha: 0.16),
                labelStyle: TextStyle(
                  color: timingPreference == slot ? AppColors.primary : null,
                  fontWeight: timingPreference == slot ? FontWeight.w700 : FontWeight.w500,
                ),
                side: BorderSide(color: timingPreference == slot ? AppColors.primary : AppColors.borderLight),
              ),
          ],
        ),
        if (requiresPropertyVisit) ...[
          const SizedBox(height: 28),
          PremiumCard(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    color: AppColors.secondary.withValues(alpha: 0.16),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                  ),
                  child: const Icon(Icons.home_work_rounded, color: AppColors.secondaryDark),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Property Verification Visit', style: theme.textTheme.titleSmall),
                      const SizedBox(height: 4),
                      Text(
                        'This loan requires an in-person property visit. Choose a convenient date and time.',
                        style: theme.textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Text('Visit Date', style: theme.textTheme.titleMedium),
          const SizedBox(height: 8),
          InkWell(
            onTap: onPickVisitDate,
            borderRadius: BorderRadius.circular(AppRadius.sm),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(AppRadius.sm),
                border: Border.all(color: AppColors.borderLight),
              ),
              child: Row(
                children: [
                  const Icon(Icons.calendar_today_rounded, size: 18, color: AppColors.primary),
                  const SizedBox(width: 10),
                  Text(
                    visitDate == null ? 'Select a visit date' : DateFormat('EEE, d MMM yyyy').format(visitDate!),
                    style: theme.textTheme.bodyMedium,
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 20),
          Text('Visit Time', style: theme.textTheme.titleMedium),
          const SizedBox(height: 12),
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              for (final slot in _timeSlots)
                ChoiceChip(
                  label: Text(slot),
                  selected: visitTimeSlot == slot,
                  onSelected: (_) => onVisitTimeSlotChanged(slot),
                  selectedColor: AppColors.secondary.withValues(alpha: 0.2),
                  labelStyle: TextStyle(
                    color: visitTimeSlot == slot ? AppColors.secondaryDark : null,
                    fontWeight: visitTimeSlot == slot ? FontWeight.w700 : FontWeight.w500,
                  ),
                  side: BorderSide(color: visitTimeSlot == slot ? AppColors.secondaryDark : AppColors.borderLight),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

/// Instant Loan-only consent step: an eye-catching, gently pulsing
/// "Repayment Due in 35 Days" heading (repeating scale animation,
/// 0.95↔1.05 over ~1.4s — noticeable, not seizure-inducing), standard
/// consent copy, and a required "I agree" checkbox.
class _InstantLoanConsentStep extends StatefulWidget {
  const _InstantLoanConsentStep({required this.agreed, required this.onAgreedChanged});

  final bool agreed;
  final ValueChanged<bool?> onAgreedChanged;

  @override
  State<_InstantLoanConsentStep> createState() => _InstantLoanConsentStepState();
}

class _InstantLoanConsentStepState extends State<_InstantLoanConsentStep> with SingleTickerProviderStateMixin {
  late final AnimationController _pulseController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1400),
  )..repeat(reverse: true);
  late final Animation<double> _pulse = Tween<double>(begin: 0.95, end: 1.05).animate(
    CurvedAnimation(parent: _pulseController, curve: Curves.easeInOut),
  );

  @override
  void dispose() {
    _pulseController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        PremiumCard(
          child: Column(
            children: [
              const SizedBox(height: 8),
              ScaleTransition(
                scale: _pulse,
                child: Text(
                  '⚠ Repayment Due in 35 Days',
                  textAlign: TextAlign.center,
                  style: theme.textTheme.headlineSmall?.copyWith(
                    color: AppColors.warning,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              const SizedBox(height: 12),
              Text(
                'This Instant Loan must be repaid in full within 35 days of disbursement. '
                'Please make sure you can repay on time before proceeding.',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium,
              ),
              const SizedBox(height: 8),
            ],
          ),
        ),
        const SizedBox(height: 20),
        Text('Instant Loan Terms', style: theme.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text(
          'By proceeding, you acknowledge and agree to the following:',
          style: theme.textTheme.bodySmall,
        ),
        const SizedBox(height: 12),
        PremiumCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '• The full loan amount, along with applicable interest and charges, is due within 35 days of disbursement.\n\n'
                '• Late repayment will attract overdue charges as disclosed in the Key Facts Statement and may affect your credit score.\n\n'
                '• Instant Loans are processed without a video KYC call — please ensure all your documents and details are accurate before submitting.\n\n'
                '• This loan is governed by the same RBI Digital Lending Guidelines and grievance redressal process as every other product in the app.',
                style: theme.textTheme.bodySmall?.copyWith(height: 1.6),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        CheckboxListTile(
          value: widget.agreed,
          onChanged: widget.onAgreedChanged,
          controlAffinity: ListTileControlAffinity.leading,
          contentPadding: EdgeInsets.zero,
          activeColor: AppColors.primary,
          title: Text(
            'I have read and agree to the loan terms.',
            style: theme.textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600),
          ),
        ),
      ],
    );
  }
}

class _TermsStep extends StatefulWidget {
  const _TermsStep({
    required this.productName,
    required this.loanAmount,
    required this.tenureMonths,
    required this.interestRate,
    required this.aprRate,
    required this.processingFee,
    required this.emi,
    required this.signatureBytes,
    required this.onSigned,
    required this.onClearSignature,
  });

  final String productName;
  final double loanAmount;
  final double tenureMonths;
  final double interestRate;
  final double aprRate;
  final String processingFee;
  final double emi;
  final Uint8List? signatureBytes;
  final ValueChanged<Uint8List> onSigned;
  final VoidCallback onClearSignature;

  @override
  State<_TermsStep> createState() => _TermsStepState();
}

class _TermsStepState extends State<_TermsStep> {
  final _scrollController = ScrollController();
  final _signatureController = SignaturePadController();
  bool _reachedBottom = false;
  bool _agreedToTerms = false;

  static const _termsSections = [
    (
      'Loan Disbursal',
      'The sanctioned loan amount will be disbursed to your registered bank account after successful completion of KYC verification, video KYC (and property verification where applicable), and internal credit approval.',
    ),
    (
      'Interest & Charges',
      'Interest is charged on a reducing balance basis at the rate stated in the Key Facts Statement above. The Annual Percentage Rate (APR) includes interest and all applicable processing charges, as mandated by RBI\'s Key Facts Statement guidelines (2024).',
    ),
    (
      'Repayment',
      'You agree to repay the loan through Equated Monthly Instalments (EMIs) as per the agreed schedule, via your registered bank account (NACH/auto-debit) or any other payment method made available in the app.',
    ),
    (
      'Overdue Charges',
      'In the event of delayed or missed EMI payments, overdue charges as stated in the Key Facts Statement will apply on the overdue amount, calculated from the due date until the date of actual payment.',
    ),
    (
      'Prepayment & Foreclosure',
      'You may prepay or foreclose the loan, in part or in full, subject to applicable prepayment charges (if any) disclosed at the time of such request, in accordance with RBI\'s Fair Practices Code.',
    ),
    (
      'Credit Bureau Reporting',
      'You authorize the lender to obtain credit information from CIBIL and other credit information companies for assessment of this application, and to report the loan account status (including any delinquency) to such bureaus on an ongoing basis.',
    ),
    (
      'Communication Consent',
      'You consent to receive application status updates, payment reminders, and other service communication via SMS, email, WhatsApp, and push notification.',
    ),
    (
      'Grievance Redressal',
      'Any grievance regarding this loan may be raised through the app\'s Support section or with the Nodal Officer, whose details are available under Compliance in the app. The lender will resolve grievances within the timelines prescribed by the RBI Fair Practices Code.',
    ),
    (
      'Governing Law',
      'This agreement is governed by the laws of India and is subject to the guidelines issued by the Reserve Bank of India for Non-Banking Financial Companies (NBFCs) and Digital Lending, as amended from time to time.',
    ),
    (
      'Data Privacy',
      'Personal and financial information collected during this application will be used solely for loan processing, underwriting, servicing, and regulatory compliance, and will not be shared with third parties except as required by law or as disclosed in the app\'s Privacy Policy.',
    ),
  ];

  @override
  void initState() {
    super.initState();
    _agreedToTerms = widget.signatureBytes != null;
    _reachedBottom = _agreedToTerms;
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.removeListener(_onScroll);
    _scrollController.dispose();
    _signatureController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_reachedBottom || !_scrollController.hasClients) return;
    final metrics = _scrollController.position;
    if (metrics.pixels >= metrics.maxScrollExtent - 24) {
      setState(() => _reachedBottom = true);
    }
  }

  Future<void> _confirmSignature() async {
    if (_signatureController.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please sign in the box above to continue.')),
      );
      return;
    }
    final bytes = await _signatureController.capture();
    if (bytes == null) return;
    widget.onSigned(bytes);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
    final isSigned = widget.signatureBytes != null;

    return Column(
      children: [
        Expanded(
          child: ListView(
            controller: _scrollController,
            padding: const EdgeInsets.all(20),
            children: [
              Text('Key Facts Statement', style: theme.textTheme.titleMedium),
              const SizedBox(height: 4),
              Text('A summary of your loan terms, as mandated by RBI.', style: theme.textTheme.bodySmall),
              const SizedBox(height: 12),
              PremiumCard(
                child: Column(
                  children: [
                    _KfsRow(label: 'Loan Product', value: widget.productName),
                    const Divider(height: 20),
                    _KfsRow(label: 'Loan Amount', value: currency.format(widget.loanAmount)),
                    const Divider(height: 20),
                    _KfsRow(label: 'Tenure', value: '${widget.tenureMonths.round()} months'),
                    const Divider(height: 20),
                    _KfsRow(label: 'Interest Rate', value: '${widget.interestRate}% p.a.'),
                    const Divider(height: 20),
                    _KfsRow(label: 'Annual Percentage Rate (APR)', value: '${widget.aprRate}% p.a.'),
                    const Divider(height: 20),
                    _KfsRow(label: 'Processing Fee', value: widget.processingFee),
                    const Divider(height: 20),
                    const _KfsRow(label: 'Overdue Charges', value: '2% p.m. on overdue amount'),
                    const Divider(height: 20),
                    _KfsRow(label: 'Estimated Monthly EMI', value: currency.format(widget.emi)),
                  ],
                ),
              ),
              const SizedBox(height: 24),
              Text('Terms & Conditions', style: theme.textTheme.titleMedium),
              const SizedBox(height: 4),
              Text('Please read the full agreement below before signing.', style: theme.textTheme.bodySmall),
              const SizedBox(height: 12),
              PremiumCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    for (int i = 0; i < _termsSections.length; i++) ...[
                      Text('${i + 1}. ${_termsSections[i].$1}', style: theme.textTheme.titleSmall),
                      const SizedBox(height: 6),
                      Text(_termsSections[i].$2, style: theme.textTheme.bodySmall?.copyWith(height: 1.5)),
                      if (i != _termsSections.length - 1) const SizedBox(height: 18),
                    ],
                  ],
                ),
              ),
              const SizedBox(height: 12),
              GestureDetector(
                onTap: () => context.push('/compliance'),
                child: Text(
                  'View Grievance Redressal & Nodal Officer details',
                  style: theme.textTheme.bodySmall?.copyWith(color: AppColors.primary, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(height: 24),
              if (!_agreedToTerms) ...[
                if (!_reachedBottom)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Text(
                      'Scroll to the end of the terms to continue.',
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodySmall?.copyWith(color: AppColors.textSecondaryLight),
                    ),
                  ),
                ElevatedButton(
                  onPressed: _reachedBottom ? () => setState(() => _agreedToTerms = true) : null,
                  style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
                  child: const Text('I Agree'),
                ),
              ] else ...[
                Text('e-Sign to Confirm', style: theme.textTheme.titleMedium),
                const SizedBox(height: 4),
                Text(
                  isSigned ? 'You\'ve signed this agreement.' : 'Sign in the box below with your finger to authorize this application.',
                  style: theme.textTheme.bodySmall,
                ),
                const SizedBox(height: 12),
                if (isSigned)
                  Container(
                    height: 180,
                    width: double.infinity,
                    decoration: BoxDecoration(
                      border: Border.all(color: AppColors.success, width: 1.4),
                      borderRadius: BorderRadius.circular(AppRadius.md),
                      color: Colors.white,
                    ),
                    child: Image.memory(widget.signatureBytes!, fit: BoxFit.contain),
                  )
                else
                  SignaturePad(controller: _signatureController),
                const SizedBox(height: 12),
                if (isSigned)
                  OutlinedButton.icon(
                    onPressed: () {
                      _signatureController.clear();
                      widget.onClearSignature();
                    },
                    icon: const Icon(Icons.edit_rounded, size: 18),
                    label: const Text('Re-sign'),
                    style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                  )
                else
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton(
                          onPressed: () => setState(() => _signatureController.clear()),
                          style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                          child: const Text('Clear'),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: ElevatedButton(
                          onPressed: _confirmSignature,
                          style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                          child: const Text('Confirm Signature'),
                        ),
                      ),
                    ],
                  ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

class _KfsRow extends StatelessWidget {
  const _KfsRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: theme.textTheme.bodyMedium),
        Flexible(child: Text(value, style: theme.textTheme.titleSmall, textAlign: TextAlign.right)),
      ],
    );
  }
}

class _SuccessView extends StatelessWidget {
  const _SuccessView({
    required this.applicationId,
    required this.productName,
    required this.loanAmount,
    required this.tenureMonths,
    required this.emi,
    required this.timingPreference,
    required this.requiresPropertyVisit,
    required this.visitDate,
    required this.visitTimeSlot,
    required this.skipVideoKyc,
    required this.showsProcessingNotice,
  });

  final String applicationId;
  final String productName;
  final double loanAmount;
  final double tenureMonths;
  final double emi;
  final String? timingPreference;
  final bool requiresPropertyVisit;
  final DateTime? visitDate;
  final String? visitTimeSlot;
  final bool skipVideoKyc;

  /// Housing Loan / Business Loan only — shows a "may take up to 1 month"
  /// processing-time notice on the success screen.
  final bool showsProcessingNotice;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);

    return Scaffold(
      appBar: AppBar(title: const Text('Application Submitted')),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            children: [
              const SuccessCelebration(),
              const SizedBox(height: 12),
              Text('Application Submitted!', style: theme.textTheme.headlineMedium, textAlign: TextAlign.center),
              const SizedBox(height: 8),
              Text(
                skipVideoKyc
                    ? 'Your application is being processed instantly — no video KYC call needed.'
                    : 'Our employee will contact you for a video KYC — please keep your documentation ready.',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium,
              ),
              const SizedBox(height: 24),
              PremiumCard(
                child: Column(
                  children: [
                    _KfsRow(label: 'Application ID', value: applicationId),
                    const Divider(height: 20),
                    _KfsRow(label: 'Loan Product', value: productName),
                    const Divider(height: 20),
                    _KfsRow(label: 'Loan Amount', value: currency.format(loanAmount)),
                    const Divider(height: 20),
                    _KfsRow(label: 'Tenure', value: '${tenureMonths.round()} months'),
                    const Divider(height: 20),
                    _KfsRow(label: 'Est. EMI', value: currency.format(emi)),
                  ],
                ),
              ),
              if (showsProcessingNotice) ...[
                const SizedBox(height: 16),
                PremiumCard(
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        width: 40,
                        height: 40,
                        decoration: BoxDecoration(
                          color: AppColors.warning.withValues(alpha: 0.14),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: const Icon(Icons.hourglass_top_rounded, color: AppColors.warning, size: 20),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('Processing May Take Time', style: theme.textTheme.titleSmall),
                            const SizedBox(height: 4),
                            Text(
                              'This process may take up to 1 month, depending on the timeliness of your document submissions.',
                              style: theme.textTheme.bodySmall,
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
              if (timingPreference != null) ...[
                const SizedBox(height: 16),
                PremiumCard(
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        width: 40,
                        height: 40,
                        decoration: BoxDecoration(
                          color: AppColors.primary.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: const Icon(Icons.video_call_rounded, color: AppColors.primary, size: 20),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('Video KYC Scheduled', style: theme.textTheme.titleSmall),
                            const SizedBox(height: 4),
                            Text('Preferred timing: $timingPreference', style: theme.textTheme.bodySmall),
                            if (requiresPropertyVisit && visitDate != null && visitTimeSlot != null) ...[
                              const SizedBox(height: 8),
                              Text(
                                'Property visit: ${DateFormat('EEE, d MMM yyyy').format(visitDate!)}, $visitTimeSlot',
                                style: theme.textTheme.bodySmall,
                              ),
                            ],
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: () => context.go('/'),
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: const Text('Done'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
