import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:printing/printing.dart';
import '../../../core/network/api_exception.dart';
import '../../../core/network/document_api_service.dart';
import '../../../core/network/loan_api_service.dart';
import '../../../core/providers/auth_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/premium_card.dart';
import '../../../core/widgets/signature_pad.dart';
import '../../../core/widgets/success_celebration.dart';
import '../../../models/loan_application.dart';
import '../../../models/uploaded_document.dart';

/// A single system-generated letter (Sanction/Disbursement) tied to a
/// loan, matched to its "Signed" counterpart if already eSigned by the
/// customer.
class _LetterKind {
  const _LetterKind({required this.docType, required this.signedDocType, required this.icon});

  final String docType; // "Sanction Letter" / "Disbursement Letter"
  final String signedDocType; // "Signed Sanction Letter" / "Signed Disbursement Letter"
  final IconData icon;
}

/// Shows "Sanction Letter" / "Disbursement Letter" actions for a loan
/// (View / Download / eSign Now), sourced from the customer's own
/// document list (`GET /auth/documents`) — the backend auto-generates
/// these as `Document` rows on sanction/disbursement
/// (`internal/loans/letter.go`). eSigning uses the real in-app signature
/// pad (see [SignaturePad]) and `POST /auth/loans/:id/esign`
/// (`internal/loans/esign_handler.go`), which embeds the drawn signature
/// directly into the regenerated letter PDF and returns the full signed
/// `Document` — including its bytes — in one round trip.
class LoanLetterActionsSection extends StatefulWidget {
  const LoanLetterActionsSection({super.key, required this.loan, this.highlightLetterType});

  final LoanApplication loan;

  /// When set (arriving via a notification deep link), the matching
  /// letter row is visually highlighted so the customer can find it
  /// immediately in a longer loan-detail sheet.
  final String? highlightLetterType;

  @override
  State<LoanLetterActionsSection> createState() => LoanLetterActionsSectionState();
}

class LoanLetterActionsSectionState extends State<LoanLetterActionsSection> {
  final GlobalKey sectionKey = GlobalKey();
  late Future<List<UploadedDocument>> _future = _load();

  Future<List<UploadedDocument>> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return const [];
    return DocumentApiService.mine(token);
  }

  Future<void> _refresh() async {
    setState(() => _future = _load());
    await _future;
  }

  List<_LetterKind> get _applicableLetters {
    final status = widget.loan.status;
    final out = <_LetterKind>[];
    if (status == 'sanctioned' || status == 'disbursed') {
      out.add(const _LetterKind(
        docType: 'Sanction Letter',
        signedDocType: 'Signed Sanction Letter',
        icon: Icons.description_rounded,
      ));
    }
    if (status == 'disbursed') {
      out.add(const _LetterKind(
        docType: 'Disbursement Letter',
        signedDocType: 'Signed Disbursement Letter',
        icon: Icons.receipt_long_rounded,
      ));
    }
    return out;
  }

  Future<void> _view(UploadedDocument doc) async {
    try {
      final bytes = base64Decode(doc.dataBase64);
      if (bytes.isEmpty) throw Exception('Empty document.');
      await Printing.layoutPdf(onLayout: (_) async => bytes);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not open this letter: $e')),
      );
    }
  }

  /// Writes the document's bytes to a real file on disk and confirms the
  /// write actually landed (non-zero length re-read) before reporting
  /// success — a silent "wrote 0 bytes" or an exception swallowed by a
  /// bare `catch (_)` is exactly the failure mode this guards against.
  Future<void> _download(UploadedDocument doc) async {
    try {
      final bytes = base64Decode(doc.dataBase64);
      if (bytes.isEmpty) {
        throw Exception('This document has no content to save.');
      }
      final dir = await getApplicationDocumentsDirectory();
      final safeName = doc.fileName.trim().isNotEmpty ? doc.fileName.trim() : '${doc.docType}.pdf';
      final file = File('${dir.path}/$safeName');
      await file.writeAsBytes(bytes, flush: true);

      // Confirm the write actually completed rather than assuming success.
      final exists = await file.exists();
      final writtenLength = exists ? await file.length() : 0;
      if (!exists || writtenLength != bytes.length) {
        throw Exception('File was not saved correctly.');
      }

      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Saved "${doc.fileName}" to app storage.'),
          action: SnackBarAction(label: 'Open', onPressed: () => _view(doc)),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not download this letter: $e')),
      );
    }
  }

  Future<void> _openEsign(_LetterKind kind) async {
    final signed = await Navigator.of(context).push<UploadedDocument>(
      MaterialPageRoute(
        builder: (_) => _EsignScreen(loan: widget.loan, kind: kind),
      ),
    );
    if (signed != null) {
      await _refresh();
    }
  }

  @override
  Widget build(BuildContext context) {
    final letters = _applicableLetters;
    if (letters.isEmpty) return const SizedBox.shrink();
    final theme = Theme.of(context);

    return FutureBuilder<List<UploadedDocument>>(
      future: _future,
      builder: (context, snapshot) {
        final docs = snapshot.data ?? const [];
        return PremiumCard(
          key: sectionKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Letters', style: theme.textTheme.titleSmall),
              const SizedBox(height: 4),
              Text(
                'Review and eSign these letters from the lender, digitally — no printing required.',
                style: theme.textTheme.bodySmall,
              ),
              const SizedBox(height: 12),
              for (int i = 0; i < letters.length; i++) ...[
                _LetterRow(
                  kind: letters[i],
                  loanId: widget.loan.id,
                  docs: docs,
                  loading: snapshot.connectionState == ConnectionState.waiting,
                  highlighted: widget.highlightLetterType == letters[i].docType,
                  onView: _view,
                  onDownload: _download,
                  onEsign: () => _openEsign(letters[i]),
                ),
                if (i != letters.length - 1) const SizedBox(height: 14),
              ],
            ],
          ),
        );
      },
    );
  }
}

class _LetterRow extends StatelessWidget {
  const _LetterRow({
    required this.kind,
    required this.loanId,
    required this.docs,
    required this.loading,
    required this.highlighted,
    required this.onView,
    required this.onDownload,
    required this.onEsign,
  });

  final _LetterKind kind;
  final int loanId;
  final List<UploadedDocument> docs;
  final bool loading;
  final bool highlighted;
  final ValueChanged<UploadedDocument> onView;
  final ValueChanged<UploadedDocument> onDownload;
  final VoidCallback onEsign;

  UploadedDocument? _find(String docType) {
    for (final d in docs) {
      if (d.docType == docType && d.loanId == loanId) return d;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final systemLetter = _find(kind.docType);
    final signedLetter = _find(kind.signedDocType);

    return AnimatedContainer(
      duration: const Duration(milliseconds: 300),
      padding: highlighted ? const EdgeInsets.all(10) : EdgeInsets.zero,
      decoration: BoxDecoration(
        color: highlighted ? AppColors.primary.withValues(alpha: 0.06) : Colors.transparent,
        borderRadius: BorderRadius.circular(AppRadius.sm),
        border: highlighted ? Border.all(color: AppColors.primary.withValues(alpha: 0.3)) : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                ),
                child: Icon(kind.icon, color: AppColors.primary, size: 18),
              ),
              const SizedBox(width: 10),
              Expanded(child: Text(kind.docType, style: theme.textTheme.titleSmall)),
            ],
          ),
          const SizedBox(height: 8),
          if (loading)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 6),
              child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
            )
          else if (systemLetter == null)
            Text('Not yet available.', style: theme.textTheme.bodySmall?.copyWith(color: AppColors.textSecondaryLight))
          else
            Wrap(
              spacing: 8,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                if (signedLetter != null) ...[
                  OutlinedButton.icon(
                    onPressed: () => onView(signedLetter),
                    icon: const Icon(Icons.visibility_rounded, size: 16),
                    label: const Text('View'),
                    style: OutlinedButton.styleFrom(visualDensity: VisualDensity.compact, minimumSize: const Size(0, 36)),
                  ),
                  OutlinedButton.icon(
                    onPressed: () => onDownload(signedLetter),
                    icon: const Icon(Icons.download_rounded, size: 16),
                    label: const Text('Download'),
                    style: OutlinedButton.styleFrom(visualDensity: VisualDensity.compact, minimumSize: const Size(0, 36)),
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                    decoration: BoxDecoration(
                      color: AppColors.success.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.check_circle_rounded, size: 16, color: AppColors.success),
                        const SizedBox(width: 6),
                        Text(
                          'eSigned ✓',
                          style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success, fontWeight: FontWeight.w700),
                        ),
                      ],
                    ),
                  ),
                  TextButton.icon(
                    onPressed: onEsign,
                    icon: const Icon(Icons.edit_rounded, size: 15),
                    label: const Text('Re-sign'),
                    style: TextButton.styleFrom(visualDensity: VisualDensity.compact, minimumSize: const Size(0, 36)),
                  ),
                ] else ...[
                  OutlinedButton.icon(
                    onPressed: () => onView(systemLetter),
                    icon: const Icon(Icons.visibility_rounded, size: 16),
                    label: const Text('View'),
                    style: OutlinedButton.styleFrom(visualDensity: VisualDensity.compact, minimumSize: const Size(0, 36)),
                  ),
                  OutlinedButton.icon(
                    onPressed: () => onDownload(systemLetter),
                    icon: const Icon(Icons.download_rounded, size: 16),
                    label: const Text('Download'),
                    style: OutlinedButton.styleFrom(visualDensity: VisualDensity.compact, minimumSize: const Size(0, 36)),
                  ),
                  ElevatedButton.icon(
                    onPressed: onEsign,
                    icon: const Icon(Icons.draw_rounded, size: 16),
                    label: const Text('eSign Now'),
                    style: ElevatedButton.styleFrom(visualDensity: VisualDensity.compact, minimumSize: const Size(0, 36)),
                  ),
                ],
              ],
            ),
        ],
      ),
    );
  }
}

/// Full-screen eSign flow for a single letter: a short recap of what's
/// being signed, the real [SignaturePad] (the same widget/capture
/// mechanism `quick_apply_screen.dart` uses for the loan agreement),
/// and a confirm step that POSTs the captured signature to
/// `/auth/loans/:id/esign` and hands back the signed `Document` on
/// success so the caller can update its state immediately — no
/// re-fetch race with the document list.
class _EsignScreen extends StatefulWidget {
  const _EsignScreen({required this.loan, required this.kind});

  final LoanApplication loan;
  final _LetterKind kind;

  @override
  State<_EsignScreen> createState() => _EsignScreenState();
}

class _EsignScreenState extends State<_EsignScreen> {
  final _signatureController = SignaturePadController();
  bool _hasStroke = false;
  bool _submitting = false;
  UploadedDocument? _result;

  @override
  void initState() {
    super.initState();
    _signatureController.addListener(_onSignatureChanged);
  }

  @override
  void dispose() {
    _signatureController.removeListener(_onSignatureChanged);
    _signatureController.dispose();
    super.dispose();
  }

  void _onSignatureChanged() {
    final hasStroke = !_signatureController.isEmpty;
    if (hasStroke != _hasStroke) setState(() => _hasStroke = hasStroke);
  }

  Future<void> _confirmEsign() async {
    if (_signatureController.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please sign in the box above to continue.')),
      );
      return;
    }
    final token = AuthProvider.instance.token;
    if (token == null) return;

    setState(() => _submitting = true);
    try {
      final pngBytes = await _signatureController.capture();
      if (pngBytes == null || pngBytes.isEmpty) {
        throw Exception('Could not capture your signature. Please try again.');
      }
      final doc = await LoanApiService.esign(
        token,
        widget.loan.id,
        letterType: widget.kind.docType,
        signaturePngBase64: base64Encode(pngBytes),
      );
      if (!mounted) return;
      setState(() {
        _result = doc;
        _submitting = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not eSign this letter: $e')),
      );
    }
  }

  Future<void> _viewSigned() async {
    final doc = _result;
    if (doc == null) return;
    try {
      final bytes = base64Decode(doc.dataBase64);
      if (bytes.isEmpty) throw Exception('Empty document.');
      await Printing.layoutPdf(onLayout: (_) async => bytes);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Could not open this letter: $e')));
    }
  }

  Future<void> _downloadSigned() async {
    final doc = _result;
    if (doc == null) return;
    try {
      final bytes = base64Decode(doc.dataBase64);
      if (bytes.isEmpty) throw Exception('This document has no content to save.');
      final dir = await getApplicationDocumentsDirectory();
      final safeName = doc.fileName.trim().isNotEmpty ? doc.fileName.trim() : '${doc.docType}.pdf';
      final file = File('${dir.path}/$safeName');
      await file.writeAsBytes(bytes, flush: true);
      final exists = await file.exists();
      final writtenLength = exists ? await file.length() : 0;
      if (!exists || writtenLength != bytes.length) {
        throw Exception('File was not saved correctly.');
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Saved "${doc.fileName}" to app storage.')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Could not download this letter: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final loan = widget.loan;
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: '₹', decimalDigits: 0);
    final isDisbursement = widget.kind.docType == 'Disbursement Letter';
    final signedDone = _result != null;

    return Scaffold(
      appBar: AppBar(title: Text(signedDone ? 'eSigned Successfully' : 'eSign ${widget.kind.docType}')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
          children: [
            if (signedDone) ...[
              Container(
                padding: const EdgeInsets.all(20),
                decoration: BoxDecoration(
                  color: AppColors.success.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppRadius.lg),
                  border: Border.all(color: AppColors.success.withValues(alpha: 0.3)),
                ),
                child: Column(
                  children: [
                    const SuccessCelebration(size: 84),
                    const SizedBox(height: 4),
                    Text('eSigned successfully', style: theme.textTheme.titleMedium),
                    const SizedBox(height: 4),
                    Text(
                      'Your ${widget.kind.docType.toLowerCase()} for Loan #${loan.id} has been signed and saved.',
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 20),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: _viewSigned,
                      icon: const Icon(Icons.visibility_rounded, size: 18),
                      label: const Text('View'),
                      style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: ElevatedButton.icon(
                      onPressed: _downloadSigned,
                      icon: const Icon(Icons.download_rounded, size: 18),
                      label: const Text('Download'),
                      style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              SizedBox(
                width: double.infinity,
                child: TextButton(
                  onPressed: () => Navigator.of(context).pop(_result),
                  child: const Text('Done'),
                ),
              ),
            ] else ...[
              Text('What you\'re signing', style: theme.textTheme.titleMedium),
              const SizedBox(height: 4),
              Text(
                'Please review this summary before signing. Your signature will be embedded directly into the official letter.',
                style: theme.textTheme.bodySmall,
              ),
              const SizedBox(height: 12),
              PremiumCard(
                child: Column(
                  children: [
                    _SummaryRow(label: 'Document', value: widget.kind.docType),
                    const Divider(height: 20),
                    _SummaryRow(label: 'Loan', value: 'NBFC-APP-${loan.id} · ${loan.category}'),
                    const Divider(height: 20),
                    _SummaryRow(
                      label: isDisbursement ? 'Disbursed Amount' : 'Requested Amount',
                      value: currency.format(isDisbursement ? loan.disbursedAmount : loan.amountRequested),
                    ),
                    const Divider(height: 20),
                    _SummaryRow(label: 'Status', value: LoanStatusInfo.forStatus(loan.status).label),
                  ],
                ),
              ),
              const SizedBox(height: 12),
              Text(
                'By signing below, you confirm that you have read and accept the terms in this letter.',
                style: theme.textTheme.bodySmall?.copyWith(color: AppColors.textSecondaryLight),
              ),
              const SizedBox(height: 20),
              Text('Your Signature', style: theme.textTheme.titleSmall),
              const SizedBox(height: 4),
              Text('Sign with your finger in the box below.', style: theme.textTheme.bodySmall),
              const SizedBox(height: 12),
              SignaturePad(controller: _signatureController),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: _hasStroke && !_submitting ? () => setState(() => _signatureController.clear()) : null,
                      style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
                      child: const Text('Clear'),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    flex: 2,
                    child: ElevatedButton(
                      onPressed: _hasStroke && !_submitting ? _confirmEsign : null,
                      style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
                      child: _submitting
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                            )
                          : const Text('Confirm & eSign'),
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _SummaryRow extends StatelessWidget {
  const _SummaryRow({required this.label, required this.value});

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
