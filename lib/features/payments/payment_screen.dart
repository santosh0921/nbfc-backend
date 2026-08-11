import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:printing/printing.dart';
import '../../core/services/notification_service.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/utils/statement_pdf_generator.dart';
import '../../core/widgets/gradient_container.dart';
import '../../core/widgets/premium_card.dart';
import '../../core/widgets/success_celebration.dart';
import '../../mock/mock_data.dart';
import '../../models/active_loan.dart';
import '../apply/widgets/apply_form_field.dart';
import 'widgets/net_banking_auth_screen.dart';
import 'widgets/payment_formatters.dart';
import 'widgets/upi_qr_scanner_screen.dart';

enum _PaymentMethod { upi, card, netBanking }

const _banks = [
  'HDFC Bank',
  'State Bank of India',
  'ICICI Bank',
  'Axis Bank',
  'Kotak Mahindra Bank',
  'Punjab National Bank',
  'Bank of Baroda',
  'IDFC FIRST Bank',
];

/// EMI Pay Now flow: pick a payment method and fill in its real form
/// (UPI ID, card number/expiry/CVV, or a bank selection for net
/// banking), confirm, "process" (mock delay), then a receipt-style
/// success screen with a generated reference number. Frontend-only — no
/// real payment gateway, nothing is sent anywhere.
class PaymentScreen extends StatefulWidget {
  const PaymentScreen({super.key});

  @override
  State<PaymentScreen> createState() => _PaymentScreenState();
}

class _PaymentScreenState extends State<PaymentScreen> {
  final _formKey = GlobalKey<FormState>();
  _PaymentMethod _method = _PaymentMethod.upi;
  bool _isProcessing = false;
  bool _isPaid = false;
  late final String _referenceNumber = 'REF${DateTime.now().millisecondsSinceEpoch}';

  final _upiController = TextEditingController();
  final _cardNumberController = TextEditingController();
  final _cardExpiryController = TextEditingController();
  final _cardCvvController = TextEditingController();
  final _cardNameController = TextEditingController();
  String? _selectedBank;
  String? _scannedUpiName;

  @override
  void dispose() {
    _upiController.dispose();
    _cardNumberController.dispose();
    _cardExpiryController.dispose();
    _cardCvvController.dispose();
    _cardNameController.dispose();
    super.dispose();
  }

  bool get _isMethodValid {
    switch (_method) {
      case _PaymentMethod.upi:
        return _upiController.text.contains('@') && _upiController.text.length > 3;
      case _PaymentMethod.card:
        return _cardNumberController.text.replaceAll(' ', '').length == 16 &&
            _cardExpiryController.text.length == 5 &&
            _cardCvvController.text.length >= 3 &&
            _cardNameController.text.trim().isNotEmpty;
      case _PaymentMethod.netBanking:
        return _selectedBank != null;
    }
  }

  Future<void> _pay() async {
    if (_method == _PaymentMethod.netBanking) {
      if (_selectedBank == null) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Please select your bank.')));
        return;
      }
      final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
      final loan = MockData.activeLoans.isNotEmpty ? MockData.activeLoans.first : null;
      final authorized = await Navigator.of(context).push<bool>(
        MaterialPageRoute(
          builder: (_) => NetBankingAuthScreen(
            bankName: _selectedBank!,
            amountLabel: currency.format(loan?.nextEmiAmount ?? 0),
          ),
        ),
      );
      if (authorized != true) return;
      await _completePayment();
      return;
    }

    if (!(_formKey.currentState?.validate() ?? false)) return;
    await _completePayment();
  }

  Future<void> _scanAndPay() async {
    final result = await Navigator.of(context).push<({String vpa, String name, String? amount, String uri})>(
      MaterialPageRoute(builder: (_) => const UpiQrScannerScreen()),
    );
    if (result == null || !mounted) return;

    setState(() {
      _upiController.text = result.vpa;
      _scannedUpiName = result.name;
    });

    final opened = await launchUpiIntent(result.uri);
    if (!mounted) return;

    if (!opened) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('No UPI app found to complete this payment.')),
      );
      return;
    }

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Complete the payment'),
        content: Text('Finish the payment to ${result.name} in your UPI app, then confirm here.'),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: const Text('Not yet')),
          ElevatedButton(onPressed: () => Navigator.of(context).pop(true), child: const Text('I\'ve Paid')),
        ],
      ),
    );
    if (confirmed == true) await _completePayment();
  }

  Future<void> _completePayment() async {
    setState(() => _isProcessing = true);
    await Future<void>.delayed(const Duration(milliseconds: 1400));
    if (!mounted) return;
    setState(() {
      _isProcessing = false;
      _isPaid = true;
    });
    final loan = MockData.activeLoans.isNotEmpty ? MockData.activeLoans.first : null;
    NotificationService.instance.show(
      title: 'Payment Successful',
      body: loan != null ? 'Your EMI payment for ${loan.productName} was received successfully.' : 'Your payment was received successfully.',
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
    final hasActiveLoan = MockData.activeLoans.isNotEmpty;
    final loan = hasActiveLoan ? MockData.activeLoans.first : null;

    if (!hasActiveLoan) {
      return Scaffold(
        appBar: AppBar(title: const Text('Payments')),
        body: const Center(child: Text('No active loans to pay.')),
      );
    }

    if (_isPaid) {
      return _PaymentSuccessView(
        loan: loan!,
        referenceNumber: _referenceNumber,
        methodSummary: _methodSummary(),
      );
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Pay EMI')),
      body: SafeArea(
        child: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(20),
            children: [
              GradientContainer(
                colors: AppColors.gradientBluePrimary,
                radius: AppRadius.xl,
                child: Stack(
                  children: [
                    Positioned(
                      right: -14,
                      top: -14,
                      child: Icon(Icons.account_balance_wallet_rounded, size: 110, color: Colors.white.withValues(alpha: 0.12)),
                    ),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text('Amount Due', style: theme.textTheme.bodyMedium?.copyWith(color: Colors.white.withValues(alpha: 0.85))),
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                              decoration: BoxDecoration(
                                color: Colors.white.withValues(alpha: 0.18),
                                borderRadius: BorderRadius.circular(AppRadius.pill),
                              ),
                              child: Text(
                                'Due ${DateFormat('d MMM').format(loan!.nextEmiDate)}',
                                style: theme.textTheme.labelSmall?.copyWith(color: Colors.white, fontWeight: FontWeight.w700),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 6),
                        Text(
                          currency.format(loan.nextEmiAmount),
                          style: theme.textTheme.displaySmall?.copyWith(color: Colors.white, fontWeight: FontWeight.w800),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          '${loan.productName} · ${loan.loanId}',
                          style: theme.textTheme.bodySmall?.copyWith(color: Colors.white.withValues(alpha: 0.85)),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 24),
              Text('Select Payment Method', style: theme.textTheme.titleMedium),
              const SizedBox(height: 12),
              _MethodTile(
                icon: Icons.qr_code_rounded,
                label: 'UPI',
                subtitle: 'Pay via any UPI app',
                selected: _method == _PaymentMethod.upi,
                onTap: () => setState(() => _method = _PaymentMethod.upi),
              ),
              if (_method == _PaymentMethod.upi) ...[
                _UpiForm(controller: _upiController, onChanged: () => setState(() => _scannedUpiName = null)),
                if (_scannedUpiName != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 6, left: 4),
                    child: Row(
                      children: [
                        const Icon(Icons.check_circle_rounded, size: 14, color: AppColors.success),
                        const SizedBox(width: 6),
                        Text('Scanned: ${_scannedUpiName!}', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success)),
                      ],
                    ),
                  ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(4, 10, 4, 0),
                  child: OutlinedButton.icon(
                    onPressed: _scanAndPay,
                    icon: const Icon(Icons.qr_code_scanner_rounded, size: 18),
                    label: const Text('Scan QR to Pay'),
                    style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                  ),
                ),
              ],
              const SizedBox(height: 10),
              _MethodTile(
                icon: Icons.credit_card_rounded,
                label: 'Debit / Credit Card',
                subtitle: 'Visa, Mastercard, RuPay',
                selected: _method == _PaymentMethod.card,
                onTap: () => setState(() => _method = _PaymentMethod.card),
              ),
              if (_method == _PaymentMethod.card)
                _CardForm(
                  numberController: _cardNumberController,
                  expiryController: _cardExpiryController,
                  cvvController: _cardCvvController,
                  nameController: _cardNameController,
                  onChanged: () => setState(() {}),
                ),
              const SizedBox(height: 10),
              _MethodTile(
                icon: Icons.account_balance_rounded,
                label: 'Net Banking',
                subtitle: 'All major banks supported',
                selected: _method == _PaymentMethod.netBanking,
                onTap: () => setState(() => _method = _PaymentMethod.netBanking),
              ),
              if (_method == _PaymentMethod.netBanking)
                _BankList(
                  selectedBank: _selectedBank,
                  onSelect: (bank) => setState(() => _selectedBank = bank),
                ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: (_isProcessing || !_isMethodValid) ? null : _pay,
                style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                child: _isProcessing
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(strokeWidth: 2.4, valueColor: AlwaysStoppedAnimation(Colors.white)),
                      )
                    : Text(
                        'Pay ${currency.format(loan.nextEmiAmount)}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _methodSummary() {
    switch (_method) {
      case _PaymentMethod.upi:
        return _scannedUpiName != null ? 'UPI (Scanned) · ${_scannedUpiName!}' : 'UPI · ${_upiController.text}';
      case _PaymentMethod.card:
        final digits = _cardNumberController.text.replaceAll(' ', '');
        final last4 = digits.length >= 4 ? digits.substring(digits.length - 4) : digits;
        return 'Card ending $last4';
      case _PaymentMethod.netBanking:
        return 'Net Banking · ${_selectedBank ?? ''}';
    }
  }
}

class _PaymentSuccessView extends StatefulWidget {
  const _PaymentSuccessView({required this.loan, required this.referenceNumber, required this.methodSummary});

  final ActiveLoan loan;
  final String referenceNumber;
  final String methodSummary;

  @override
  State<_PaymentSuccessView> createState() => _PaymentSuccessViewState();
}

class _PaymentSuccessViewState extends State<_PaymentSuccessView> with SingleTickerProviderStateMixin {
  /// Total repayable over the full tenure at a constant EMI — the same
  /// principal-plus-interest approximation used elsewhere in the app
  /// (see internal/loans/letter.go's "Estimated Total Repayment").
  double get _totalPayable => widget.loan.nextEmiAmount * widget.loan.tenureMonths;
  double get _totalInterest => _totalPayable - widget.loan.principal;

  Future<File> _buildReceiptFile() async {
    final bytes = await StatementPdfGenerator.buildPaymentReceipt(
      referenceNumber: widget.referenceNumber,
      loanProductName: widget.loan.productName,
      loanId: widget.loan.loanId,
      amount: widget.loan.nextEmiAmount,
      date: DateTime.now(),
      paymentMethod: widget.methodSummary,
      status: 'Success',
      loanAmount: widget.loan.principal,
      interestAmount: _totalInterest,
      totalPayable: _totalPayable,
    );
    final dir = await getApplicationDocumentsDirectory();
    final file = File('${dir.path}/receipt_${widget.referenceNumber}.pdf');
    await file.writeAsBytes(bytes);
    return file;
  }

  Future<void> _shareReceipt() async {
    final file = await _buildReceiptFile();
    await Printing.sharePdf(bytes: await file.readAsBytes(), filename: 'receipt_${widget.referenceNumber}.pdf');
  }

  Future<void> _downloadReceipt() async {
    final file = await _buildReceiptFile();
    if (!mounted) return;
    await Printing.layoutPdf(onLayout: (_) async => file.readAsBytesSync());
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Receipt saved to ${file.path}')),
    );
  }

  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 900),
  )..forward();

  late final Animation<double> _amountFade = CurvedAnimation(parent: _controller, curve: const Interval(0.3, 0.7, curve: Curves.easeOut));
  late final Animation<Offset> _amountSlide = Tween(begin: const Offset(0, 0.2), end: Offset.zero)
      .animate(CurvedAnimation(parent: _controller, curve: const Interval(0.3, 0.7, curve: Curves.easeOutCubic)));
  late final Animation<double> _receiptFade = CurvedAnimation(parent: _controller, curve: const Interval(0.55, 1.0, curve: Curves.easeOut));
  late final Animation<Offset> _receiptSlide = Tween(begin: const Offset(0, 0.15), end: Offset.zero)
      .animate(CurvedAnimation(parent: _controller, curve: const Interval(0.55, 1.0, curve: Curves.easeOutCubic)));

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
    final date = DateFormat('d MMM yyyy, h:mm a').format(DateTime.now());

    return Scaffold(
      backgroundColor: AppColors.backgroundLight,
      appBar: AppBar(title: const Text('Payment Successful')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(24, 12, 24, 24),
          children: [
            const Center(child: SuccessCelebration()),
            FadeTransition(
              opacity: _amountFade,
              child: SlideTransition(
                position: _amountSlide,
                child: Column(
                  children: [
                    Text(
                      currency.format(widget.loan.nextEmiAmount),
                      style: theme.textTheme.displayMedium?.copyWith(fontWeight: FontWeight.w800, color: AppColors.success),
                    ),
                    const SizedBox(height: 4),
                    Text('Payment successful', style: theme.textTheme.titleSmall),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 28),
            FadeTransition(
              opacity: _receiptFade,
              child: SlideTransition(
                position: _receiptSlide,
                child: Column(
                  children: [
                    PremiumCard(
                      child: Column(
                        children: [
                          _ReceiptRow(label: 'Loan', value: widget.loan.productName),
                          const Divider(height: 20),
                          _ReceiptRow(label: 'Loan ID', value: widget.loan.loanId),
                          const Divider(height: 20),
                          _ReceiptRow(label: 'Reference No.', value: widget.referenceNumber),
                          const Divider(height: 20),
                          _ReceiptRow(label: 'Date & Time', value: date),
                          const Divider(height: 20),
                          _ReceiptRow(label: 'Payment Method', value: widget.methodSummary),
                          const Divider(height: 20),
                          _ReceiptRow(label: 'Loan Amount', value: currency.format(widget.loan.principal)),
                          const Divider(height: 20),
                          _ReceiptRow(label: 'Interest Amount', value: currency.format(_totalInterest)),
                          const Divider(height: 20),
                          _ReceiptRow(
                            label: 'Total Amount (with Interest)',
                            value: currency.format(_totalPayable),
                            emphasize: true,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 16),
                    Row(
                      children: [
                        Expanded(
                          child: OutlinedButton.icon(
                            onPressed: _shareReceipt,
                            icon: const Icon(Icons.share_rounded, size: 18),
                            label: const Text('Share'),
                            style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: OutlinedButton.icon(
                            onPressed: _downloadReceipt,
                            icon: const Icon(Icons.download_rounded, size: 18),
                            label: const Text('Download'),
                            style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 16),
                    ElevatedButton(
                      onPressed: () => context.go('/'),
                      style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(54)),
                      child: const Text('Done'),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _UpiForm extends StatelessWidget {
  const _UpiForm({required this.controller, required this.onChanged});

  final TextEditingController controller;
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 12, 4, 0),
      child: ApplyFormField(
        label: 'UPI ID',
        controller: controller,
        hint: 'yourname@okhdfcbank',
        validator: (v) => (v == null || !v.contains('@')) ? 'Enter a valid UPI ID' : null,
        onChanged: (_) => onChanged(),
      ),
    );
  }
}

class _CardForm extends StatelessWidget {
  const _CardForm({
    required this.numberController,
    required this.expiryController,
    required this.cvvController,
    required this.nameController,
    required this.onChanged,
  });

  final TextEditingController numberController;
  final TextEditingController expiryController;
  final TextEditingController cvvController;
  final TextEditingController nameController;
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 12, 4, 0),
      child: Column(
        children: [
          ApplyFormField(
            label: 'Card Number',
            controller: numberController,
            keyboardType: TextInputType.number,
            hint: '1234 5678 9012 3456',
            maxLength: 19,
            inputFormatters: [CardNumberInputFormatter()],
            validator: (v) => (v == null || v.replaceAll(' ', '').length != 16) ? 'Enter a valid 16-digit card number' : null,
            onChanged: (_) => onChanged(),
          ),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: ApplyFormField(
                  label: 'Expiry (MM/YY)',
                  controller: expiryController,
                  keyboardType: TextInputType.number,
                  hint: 'MM/YY',
                  maxLength: 5,
                  inputFormatters: [ExpiryDateInputFormatter()],
                  validator: (v) => (v == null || v.length != 5) ? 'Invalid' : null,
                  onChanged: (_) => onChanged(),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: ApplyFormField(
                  label: 'CVV',
                  controller: cvvController,
                  keyboardType: TextInputType.number,
                  hint: '123',
                  maxLength: 4,
                  obscureText: true,
                  inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                  validator: (v) => (v == null || v.length < 3) ? 'Invalid' : null,
                  onChanged: (_) => onChanged(),
                ),
              ),
            ],
          ),
          ApplyFormField(
            label: 'Name on Card',
            controller: nameController,
            validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter the name on your card' : null,
            onChanged: (_) => onChanged(),
          ),
        ],
      ),
    );
  }
}

class _BankList extends StatelessWidget {
  const _BankList({required this.selectedBank, required this.onSelect});

  final String? selectedBank;
  final ValueChanged<String> onSelect;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 12, 4, 0),
      child: PremiumCard(
        padding: EdgeInsets.zero,
        child: Column(
          children: [
            for (int i = 0; i < _banks.length; i++) ...[
              ListTile(
                onTap: () => onSelect(_banks[i]),
                title: Text(_banks[i], style: theme.textTheme.titleSmall),
                trailing: Icon(
                  selectedBank == _banks[i] ? Icons.radio_button_checked_rounded : Icons.radio_button_off_rounded,
                  color: selectedBank == _banks[i] ? AppColors.primary : AppColors.textSecondaryLight,
                ),
              ),
              if (i != _banks.length - 1) const Divider(height: 1, indent: 16),
            ],
          ],
        ),
      ),
    );
  }
}

class _MethodTile extends StatelessWidget {
  const _MethodTile({
    required this.icon,
    required this.label,
    required this.subtitle,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final String subtitle;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return PremiumCard(
      onTap: onTap,
      radius: AppRadius.md,
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            child: Icon(icon, color: AppColors.primary),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: theme.textTheme.titleSmall),
                Text(subtitle, style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          Icon(
            selected ? Icons.radio_button_checked_rounded : Icons.radio_button_off_rounded,
            color: selected ? AppColors.primary : AppColors.textSecondaryLight,
          ),
        ],
      ),
    );
  }
}

class _ReceiptRow extends StatelessWidget {
  const _ReceiptRow({required this.label, required this.value, this.emphasize = false});

  final String label;
  final String value;
  final bool emphasize;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: emphasize ? theme.textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w700) : theme.textTheme.bodyMedium,
        ),
        Flexible(
          child: Text(
            value,
            style: emphasize
                ? theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800, color: AppColors.primary)
                : theme.textTheme.titleSmall,
            textAlign: TextAlign.right,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }
}
