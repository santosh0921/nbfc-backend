import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:geolocator/geolocator.dart';
import 'package:image_picker/image_picker.dart';
import 'package:latlong2/latlong.dart' as latlong;
import 'package:path_provider/path_provider.dart';
import 'package:provider/provider.dart';
import 'package:uuid/uuid.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../../../../core/widgets/signature_pad.dart';
import '../../../cases/domain/repositories/cases_repository.dart';
import '../../domain/entities/visit_record.dart';
import '../providers/verification_draft_provider.dart';
import 'verification_success_page.dart';

/// End-to-end field verification: customer & loan details -> witnesses ->
/// document capture -> geotagged site photo -> e-signatures (customer +
/// officer) -> review & submit. Mirrors the real-world visit workflow that
/// starts once a customer agrees to proceed during the pre-visit call.
class VerificationFlowPage extends StatefulWidget {
  const VerificationFlowPage({
    super.key,
    required this.caseId,
    required this.customerName,
    required this.loanType,
    this.documentChecklist = const ['Aadhaar Card', 'PAN Card', 'Salary Slip', 'Bank Statement'],
    this.customerPhone,
    this.timingPreference,
  });

  final String caseId;
  final String customerName;
  final String loanType;
  final List<String> documentChecklist;

  /// Threaded through from the case so it's still visible throughout the
  /// visit, not just on the case detail screen before the call.
  final String? customerPhone;
  final String? timingPreference;

  @override
  State<VerificationFlowPage> createState() => _VerificationFlowPageState();
}

class _VerificationFlowPageState extends State<VerificationFlowPage> {
  int _step = 0;
  static const _stepTitles = ['Details', 'Witnesses', 'Documents', 'Site Photo', 'Sign-off', 'Review'];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<VerificationDraftProvider>().startVisit(
            caseId: widget.caseId,
            customerName: widget.customerName,
            loanType: widget.loanType,
          );
    });
  }

  void _goTo(int step) => setState(() => _step = step);

  void _next() {
    if (_step < _stepTitles.length - 1) _goTo(_step + 1);
  }

  void _back() {
    if (_step > 0) {
      _goTo(_step - 1);
    } else {
      Navigator.of(context).pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        leading: IconButton(icon: const Icon(Icons.arrow_back), onPressed: _back),
        title: Text('Verification · ${widget.customerName}', maxLines: 1, overflow: TextOverflow.ellipsis),
      ),
      body: Column(
        children: [
          _StepIndicator(titles: _stepTitles, current: _step),
          Expanded(
            // IndexedStack keeps every step built (not lazily rendered like
            // a scrolling PageView viewport), so a step can never come up
            // blank — each step's own AnimatedSwitcher-free content is
            // simply shown/hidden instantly.
            child: IndexedStack(
              index: _step,
              children: [
                _DetailsStep(onNext: _next, customerPhone: widget.customerPhone, timingPreference: widget.timingPreference),
                _WitnessesStep(onNext: _next),
                _DocumentsStep(checklist: widget.documentChecklist, onNext: _next),
                _GeoPhotoStep(onNext: _next),
                _SignOffStep(onNext: _next),
                const _ReviewStep(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _StepIndicator extends StatelessWidget {
  const _StepIndicator({required this.titles, required this.current});

  final List<String> titles;
  final int current;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
      child: Row(
        children: [
          for (int i = 0; i < titles.length; i++) ...[
            Expanded(
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 220),
                height: 4,
                decoration: BoxDecoration(
                  color: i <= current ? AppColors.primary : AppColors.border,
                  borderRadius: BorderRadius.circular(AppSpacing.radiusFull),
                ),
              ),
            ),
            if (i != titles.length - 1) const SizedBox(width: 4),
          ],
        ],
      ),
    );
  }
}

class _StepScaffold extends StatelessWidget {
  const _StepScaffold({required this.title, required this.child, this.footer});

  final String title;
  final Widget child;
  final Widget? footer;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.md),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: Theme.of(context).textTheme.headlineSmall),
                const SizedBox(height: AppSpacing.md),
                child,
              ],
            ),
          ),
        ),
        if (footer != null) Padding(padding: const EdgeInsets.all(AppSpacing.md), child: footer!),
      ],
    );
  }
}

class _DetailsStep extends StatefulWidget {
  const _DetailsStep({required this.onNext, this.customerPhone, this.timingPreference});

  final VoidCallback onNext;
  final String? customerPhone;
  final String? timingPreference;

  @override
  State<_DetailsStep> createState() => _DetailsStepState();
}

class _DetailsStepState extends State<_DetailsStep> {
  final _occupationController = TextEditingController();
  final _incomeController = TextEditingController();
  final _remarksController = TextEditingController();

  @override
  void dispose() {
    _occupationController.dispose();
    _incomeController.dispose();
    _remarksController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final draft = context.read<VerificationDraftProvider>();

    return _StepScaffold(
      title: 'Customer & Loan Details',
      footer: AppButton(
        label: 'Next: Witnesses',
        onPressed: () {
          draft.updateDetails(
            occupation: _occupationController.text.trim(),
            monthlyIncome: _incomeController.text.trim(),
            remarks: _remarksController.text.trim(),
          );
          widget.onNext();
        },
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Customer', style: Theme.of(context).textTheme.bodySmall),
          Text(draft.customerName, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(draft.loanType, style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
          if (widget.customerPhone != null && widget.customerPhone!.isNotEmpty) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                const Icon(Icons.phone_outlined, size: 16, color: AppColors.textSecondary),
                const SizedBox(width: 6),
                Text(widget.customerPhone!, style: Theme.of(context).textTheme.bodyMedium),
              ],
            ),
          ],
          if (widget.timingPreference != null && widget.timingPreference!.isNotEmpty) ...[
            const SizedBox(height: 6),
            Row(
              children: [
                const Icon(Icons.schedule_outlined, size: 16, color: AppColors.textSecondary),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    'Preferred call time: ${widget.timingPreference}',
                    style: Theme.of(context).textTheme.bodyMedium,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ],
          const SizedBox(height: AppSpacing.lg),
          AppTextField(label: 'Occupation', hint: 'Enter occupation', controller: _occupationController, prefixIcon: Icons.work_outline),
          const SizedBox(height: AppSpacing.md),
          AppTextField(
            label: 'Monthly Income (₹)',
            controller: _incomeController,
            prefixIcon: Icons.currency_rupee,
            keyboardType: TextInputType.number,
          ),
          const SizedBox(height: AppSpacing.md),
          AppTextField(label: 'Field Remarks', hint: 'Enter field remarks', controller: _remarksController, prefixIcon: Icons.notes_outlined),
        ],
      ),
    );
  }
}

class _WitnessesStep extends StatefulWidget {
  const _WitnessesStep({required this.onNext});

  final VoidCallback onNext;

  @override
  State<_WitnessesStep> createState() => _WitnessesStepState();
}

class _WitnessesStepState extends State<_WitnessesStep> {
  final _nameController = TextEditingController();
  final _relationController = TextEditingController();
  final _phoneController = TextEditingController();
  final _dobController = TextEditingController();
  bool _capturingDocFor = false;

  void _addWitness() {
    if (_nameController.text.trim().isEmpty) {
      AppSnackbar.danger(context, 'Enter witness name');
      return;
    }
    context.read<VerificationDraftProvider>().addWitness(WitnessInfo(
          name: _nameController.text.trim(),
          relation: _relationController.text.trim(),
          phone: _phoneController.text.trim(),
          dob: _dobController.text.trim(),
        ));
    _nameController.clear();
    _relationController.clear();
    _phoneController.clear();
    _dobController.clear();
    setState(() {});
  }

  Future<void> _captureWitnessAadhaar(int index) async {
    setState(() => _capturingDocFor = true);
    try {
      final photo = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) {
        await context.read<VerificationDraftProvider>().addWitnessAadhaar(index, photo.path);
      }
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture Aadhaar card');
    } finally {
      if (mounted) setState(() => _capturingDocFor = false);
    }
  }

  Future<void> _captureWitnessPan(int index) async {
    setState(() => _capturingDocFor = true);
    try {
      final photo = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) {
        await context.read<VerificationDraftProvider>().addWitnessPan(index, photo.path);
      }
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture PAN card');
    } finally {
      if (mounted) setState(() => _capturingDocFor = false);
    }
  }

  Future<void> _captureWitnessSignature(int index) async {
    final bytes = await SignaturePad.captureFullscreen(context);
    if (bytes == null || !mounted) return;
    setState(() => _capturingDocFor = true);
    try {
      final dir = await getApplicationDocumentsDirectory();
      final file = File('${dir.path}/witness_${index}_sig_${const Uuid().v4()}.png');
      await file.writeAsBytes(bytes);
      if (mounted) await context.read<VerificationDraftProvider>().addWitnessSignature(index, file.path);
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not save witness signature');
    } finally {
      if (mounted) setState(() => _capturingDocFor = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final draft = context.watch<VerificationDraftProvider>();

    return _StepScaffold(
      title: 'Witness Details',
      footer: AppButton(label: 'Next: Documents', onPressed: widget.onNext),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ...draft.witnesses.asMap().entries.map((entry) => AppCard(
                padding: const EdgeInsets.all(AppSpacing.sm),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text(entry.value.name),
                      subtitle: Text('${entry.value.relation} · ${entry.value.phone} · DOB ${entry.value.dob}'),
                      trailing: IconButton(
                        icon: const Icon(Icons.delete_outline, color: AppColors.danger),
                        onPressed: () => context.read<VerificationDraftProvider>().removeWitness(entry.key),
                      ),
                    ),
                    Padding(
                      padding: const EdgeInsets.only(left: AppSpacing.sm, bottom: AppSpacing.xs),
                      child: Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          _WitnessCaptureChip(
                            label: 'Aadhaar',
                            done: entry.value.aadhaarPath != null,
                            filePath: entry.value.aadhaarPath,
                            icon: Icons.badge_outlined,
                            isBusy: _capturingDocFor,
                            onTap: () => _captureWitnessAadhaar(entry.key),
                          ),
                          _WitnessCaptureChip(
                            label: 'PAN',
                            done: entry.value.panPath != null,
                            filePath: entry.value.panPath,
                            icon: Icons.credit_card_outlined,
                            isBusy: _capturingDocFor,
                            onTap: () => _captureWitnessPan(entry.key),
                          ),
                          _WitnessCaptureChip(
                            label: 'Signature',
                            done: entry.value.signaturePath != null,
                            filePath: entry.value.signaturePath,
                            icon: Icons.draw_outlined,
                            isBusy: _capturingDocFor,
                            onTap: () => _captureWitnessSignature(entry.key),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              )),
          if (draft.witnesses.isNotEmpty) const SizedBox(height: AppSpacing.sm),
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('Add Witness', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                AppTextField(label: 'Full Name', hint: 'Enter witness full name', controller: _nameController),
                const SizedBox(height: AppSpacing.sm),
                AppTextField(label: 'Relation', hint: 'e.g. Neighbour, Colleague', controller: _relationController),
                const SizedBox(height: AppSpacing.sm),
                AppTextField(label: 'Phone', hint: 'Enter phone number', controller: _phoneController, keyboardType: TextInputType.phone),
                const SizedBox(height: AppSpacing.sm),
                AppTextField(label: 'Date of Birth', controller: _dobController, hint: 'DD/MM/YYYY'),
                const SizedBox(height: AppSpacing.sm),
                AppButton(label: 'Add Witness', variant: AppButtonVariant.outline, icon: Icons.add, onPressed: _addWitness),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// One capture action for a single witness's Aadhaar/PAN/signature —
/// shows a captured thumbnail (or a signature icon, since a signature PNG
/// makes a poor thumbnail) and a green check once done, or an outlined
/// "capture" chip beforehand.
class _WitnessCaptureChip extends StatelessWidget {
  const _WitnessCaptureChip({
    required this.label,
    required this.done,
    required this.filePath,
    required this.icon,
    required this.isBusy,
    required this.onTap,
  });

  final String label;
  final bool done;
  final String? filePath;
  final IconData icon;
  final bool isBusy;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    if (!done) {
      return AppButton(
        label: 'Capture $label',
        size: AppButtonSize.small,
        expanded: false,
        variant: AppButtonVariant.outline,
        icon: icon,
        isLoading: isBusy,
        onPressed: onTap,
      );
    }
    return InkWell(
      onTap: isBusy ? null : onTap,
      borderRadius: BorderRadius.circular(AppSpacing.radiusSm),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
        decoration: BoxDecoration(
          color: AppColors.success.withValues(alpha: 0.08),
          borderRadius: BorderRadius.circular(AppSpacing.radiusSm),
          border: Border.all(color: AppColors.success.withValues(alpha: 0.3)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (label != 'Signature' && filePath != null)
              ClipRRect(
                borderRadius: BorderRadius.circular(4),
                child: Image.file(File(filePath!), width: 22, height: 22, fit: BoxFit.cover),
              )
            else
              const Icon(Icons.check_circle, color: AppColors.success, size: 16),
            const SizedBox(width: 6),
            Text('$label ✓', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.success, fontWeight: FontWeight.w600)),
          ],
        ),
      ),
    );
  }
}

class _DocumentsStep extends StatefulWidget {
  const _DocumentsStep({required this.checklist, required this.onNext});

  final List<String> checklist;
  final VoidCallback onNext;

  @override
  State<_DocumentsStep> createState() => _DocumentsStepState();
}

class _DocumentsStepState extends State<_DocumentsStep> {
  final _picker = ImagePicker();
  bool _capturing = false;

  Future<void> _capture(String type) async {
    setState(() => _capturing = true);
    try {
      final photo = await _picker.pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) {
        final draft = context.read<VerificationDraftProvider>();
        draft.addDocument(CapturedDocument(type: type, filePath: photo.path));
        // Fire-and-forget upload to the backend so admin can see it too — the
        // local capture (above) and the local PDF-embedding flow are the
        // source of truth for the on-device UI and must keep working even if
        // this fails, so failures are surfaced as a non-blocking toast only.
        unawaited(_uploadToBackend(draft.caseId, type, photo));
      }
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture $type');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  Future<void> _uploadToBackend(String loanId, String type, XFile photo) async {
    if (!mounted) return;
    final casesRepository = context.read<CasesRepository>();
    try {
      final bytes = await File(photo.path).readAsBytes();
      await casesRepository.uploadLoanDocument(
            loanId,
            docType: type,
            fileName: photo.name,
            mimeType: 'image/jpeg',
            dataBase64: base64Encode(bytes),
          );
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Saved locally, but could not sync $type to server');
    }
  }

  @override
  Widget build(BuildContext context) {
    final draft = context.watch<VerificationDraftProvider>();
    final capturedTypes = draft.documents.map((d) => d.type).toSet();
    final allCaptured = capturedTypes.length >= widget.checklist.length;

    return _StepScaffold(
      title: 'Document Verification',
      footer: AppButton(
        label: allCaptured ? 'Next: Site Photo' : 'Next (${capturedTypes.length}/${widget.checklist.length} captured)',
        onPressed: widget.onNext,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Capture each document with the camera. You can continue and come back to finish these later.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.sm),
          for (final type in widget.checklist)
            Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: AppCard(
                child: Row(
                  children: [
                    if (capturedTypes.contains(type))
                      ClipRRect(
                        borderRadius: BorderRadius.circular(AppSpacing.radiusSm),
                        child: Image.file(
                          File(draft.documents.firstWhere((d) => d.type == type).filePath),
                          width: 44,
                          height: 44,
                          fit: BoxFit.cover,
                          errorBuilder: (context, error, stackTrace) => Container(
                            width: 44,
                            height: 44,
                            color: AppColors.surfaceMuted,
                            child: const Icon(Icons.check_circle, color: AppColors.success, size: 20),
                          ),
                        ),
                      )
                    else
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(color: AppColors.surfaceMuted, borderRadius: BorderRadius.circular(AppSpacing.radiusSm)),
                        child: const Icon(Icons.description_outlined, color: AppColors.textSecondary),
                      ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Text(type, style: Theme.of(context).textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                    ),
                    if (capturedTypes.contains(type))
                      const Icon(Icons.check_circle, color: AppColors.success)
                    else
                      AppButton(
                        label: 'Capture',
                        size: AppButtonSize.small,
                        expanded: false,
                        isLoading: _capturing,
                        icon: Icons.camera_alt_outlined,
                        onPressed: () => _capture(type),
                      ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _GeoPhotoStep extends StatefulWidget {
  const _GeoPhotoStep({required this.onNext});

  final VoidCallback onNext;

  @override
  State<_GeoPhotoStep> createState() => _GeoPhotoStepState();
}

class _GeoPhotoStepState extends State<_GeoPhotoStep> {
  bool _capturing = false;

  Future<void> _captureGeoPhoto() async {
    setState(() => _capturing = true);
    try {
      final permission = await Geolocator.checkPermission();
      var granted = permission;
      if (permission == LocationPermission.denied) {
        granted = await Geolocator.requestPermission();
      }
      if (granted == LocationPermission.denied || granted == LocationPermission.deniedForever) {
        if (mounted) AppSnackbar.danger(context, 'Location permission is required for a geotagged photo');
        return;
      }
      if (!await Geolocator.isLocationServiceEnabled()) {
        if (mounted) AppSnackbar.danger(context, 'Please enable location services');
        return;
      }

      final position = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(accuracy: LocationAccuracy.high),
      );
      final photo = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 75);
      if (photo == null || !mounted) return;

      context.read<VerificationDraftProvider>().setGeoPhoto(GeoPhoto(
            filePath: photo.path,
            latitude: position.latitude,
            longitude: position.longitude,
            capturedAt: DateTime.now(),
          ));
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture geotagged photo');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final draft = context.watch<VerificationDraftProvider>();
    final photo = draft.geoPhoto;

    return _StepScaffold(
      title: 'Geotagged Site Photo',
      footer: AppButton(
        label: photo == null ? 'Skip for now' : 'Next: Sign-off',
        variant: photo == null ? AppButtonVariant.outline : AppButtonVariant.primary,
        onPressed: widget.onNext,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Capture a photo at the customer site — your current location will be stamped automatically.',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.md),
          if (photo != null)
            AppCard(
              padding: EdgeInsets.zero,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  ClipRRect(
                    borderRadius: const BorderRadius.vertical(top: Radius.circular(AppSpacing.radiusCard)),
                    child: Image.file(
                      File(photo.filePath),
                      height: 220,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) => Container(
                        height: 220,
                        color: AppColors.surfaceMuted,
                        alignment: Alignment.center,
                        child: const Icon(Icons.image_outlined, color: AppColors.textSecondary, size: 40),
                      ),
                    ),
                  ),
                  Padding(
                    padding: const EdgeInsets.all(AppSpacing.sm),
                    child: Text(
                      'Lat ${photo.latitude.toStringAsFixed(6)}, Lng ${photo.longitude.toStringAsFixed(6)}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                  ClipRRect(
                    borderRadius: const BorderRadius.vertical(bottom: Radius.circular(AppSpacing.radiusCard)),
                    child: SizedBox(
                      height: 140,
                      child: IgnorePointer(
                        child: FlutterMap(
                          options: MapOptions(
                            initialCenter: latlong.LatLng(photo.latitude, photo.longitude),
                            initialZoom: 15,
                            interactionOptions: const InteractionOptions(flags: InteractiveFlag.none),
                          ),
                          children: [
                            TileLayer(
                              urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                              userAgentPackageName: 'com.onefin.onefin_employee',
                            ),
                            MarkerLayer(markers: [
                              Marker(
                                point: latlong.LatLng(photo.latitude, photo.longitude),
                                width: 34,
                                height: 34,
                                child: const Icon(Icons.location_on, color: AppColors.danger, size: 34),
                              ),
                            ]),
                          ],
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          const SizedBox(height: AppSpacing.md),
          AppButton(
            label: photo == null ? 'Capture Geotagged Photo' : 'Retake Photo',
            icon: Icons.add_a_photo_outlined,
            variant: AppButtonVariant.outline,
            isLoading: _capturing,
            onPressed: _captureGeoPhoto,
          ),
        ],
      ),
    );
  }
}

class _SignOffStep extends StatefulWidget {
  const _SignOffStep({required this.onNext});

  final VoidCallback onNext;

  @override
  State<_SignOffStep> createState() => _SignOffStepState();
}

class _SignOffStepState extends State<_SignOffStep> {
  final _customerSignature = SignaturePadController();
  final _employeeSignature = SignaturePadController();

  @override
  void dispose() {
    _customerSignature.dispose();
    _employeeSignature.dispose();
    super.dispose();
  }

  Future<String?> _saveSignature(SignaturePadController controller, String name) async {
    if (controller.isEmpty) return null;
    final bytes = await controller.capture();
    if (bytes == null) return null;
    final dir = await getApplicationDocumentsDirectory();
    final file = File('${dir.path}/${name}_${const Uuid().v4()}.png');
    await file.writeAsBytes(bytes);
    return file.path;
  }

  Future<void> _continue() async {
    final draft = context.read<VerificationDraftProvider>();
    final customerPath = await _saveSignature(_customerSignature, 'customer_sig');
    final employeePath = await _saveSignature(_employeeSignature, 'employee_sig');

    if (customerPath == null || employeePath == null) {
      if (mounted) AppSnackbar.danger(context, 'Both signatures are required');
      return;
    }
    draft.setCustomerSignature(customerPath);
    draft.setEmployeeSignature(employeePath);
    if (mounted) widget.onNext();
  }

  @override
  Widget build(BuildContext context) {
    return _StepScaffold(
      title: 'Sign-off',
      footer: AppButton(label: 'Next: Review', onPressed: _continue),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Customer Signature', style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: AppSpacing.sm),
          _SignaturePad(controller: _customerSignature),
          const SizedBox(height: AppSpacing.lg),
          Text('Field Officer Signature', style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: AppSpacing.sm),
          _SignaturePad(controller: _employeeSignature),
        ],
      ),
    );
  }
}

class _SignaturePad extends StatelessWidget {
  const _SignaturePad({required this.controller});

  final SignaturePadController controller;

  @override
  Widget build(BuildContext context) {
    return SignaturePad(controller: controller);
  }
}

class _ReviewStep extends StatelessWidget {
  const _ReviewStep();

  static const _recommendations = ['Approve for Review', 'Reject', 'Request Additional Documents'];
  static const _riskLevels = ['Low', 'Medium', 'High'];

  Future<void> _submit(BuildContext context) async {
    final draft = context.read<VerificationDraftProvider>();
    final record = await draft.submit();
    if (context.mounted) {
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => VerificationSuccessPage(record: record)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final draft = context.watch<VerificationDraftProvider>();

    return _StepScaffold(
      title: 'Review & Submit',
      footer: AppButton(
        label: 'Submit Report',
        isLoading: draft.isSubmitting,
        onPressed: () => _submit(context),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Verification Checklist', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                _ReviewRow(label: 'Customer', value: draft.customerName),
                _ReviewRow(label: 'Occupation', value: draft.occupation),
                _ReviewRow(label: 'Monthly Income', value: '₹${draft.monthlyIncome}'),
                _ReviewRow(label: 'Witnesses Added', value: '${draft.witnesses.length}'),
                _ReviewRow(label: 'Documents Captured', value: '${draft.documents.length}'),
                _ReviewRow(label: 'Site Photo', value: draft.geoPhoto != null ? 'Captured' : 'Missing'),
                _ReviewRow(label: 'Signatures', value: draft.customerSignaturePath != null ? 'Captured' : 'Missing'),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Risk Assessment', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                Wrap(
                  spacing: AppSpacing.sm,
                  children: _riskLevels.map((r) {
                    return ChoiceChip(
                      label: Text(r),
                      selected: draft.riskAssessment == r,
                      onSelected: (_) => context.read<VerificationDraftProvider>().setRiskAssessment(r),
                    );
                  }).toList(),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Recommendation', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                RadioGroup<String>(
                  groupValue: draft.recommendation,
                  onChanged: (value) => context.read<VerificationDraftProvider>().setRecommendation(value!),
                  child: Column(
                    children: [
                      for (final r in _recommendations)
                        RadioListTile<String>(
                          contentPadding: EdgeInsets.zero,
                          title: Text(r),
                          value: r,
                          activeColor: AppColors.secondary,
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ReviewRow extends StatelessWidget {
  const _ReviewRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Flexible(
            child: Text(label, style: Theme.of(context).textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
          ),
          const SizedBox(width: AppSpacing.sm),
          Flexible(
            child: Text(
              value.isEmpty ? '—' : value,
              style: Theme.of(context).textTheme.bodyMedium,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              textAlign: TextAlign.end,
            ),
          ),
        ],
      ),
    );
  }
}
