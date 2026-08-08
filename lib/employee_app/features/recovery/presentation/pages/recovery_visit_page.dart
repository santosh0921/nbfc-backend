import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:geolocator/geolocator.dart';
import 'package:image_picker/image_picker.dart';
import 'package:latlong2/latlong.dart' as latlong;
import 'package:path_provider/path_provider.dart';
import 'package:provider/provider.dart';
import 'package:signature/signature.dart';
import 'package:uuid/uuid.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../domain/entities/recovery_case_entity.dart';
import '../providers/recovery_cases_provider.dart';
import '../providers/recovery_visit_provider.dart';
import 'recovery_document_success_page.dart';

/// Field-visit check-in wizard for a recovery case — GPS check-in, customer
/// presence, photo/document capture, signature — mirrors the structure of
/// VerificationFlowPage but tailored to the recovery workflow.
class RecoveryVisitPage extends StatefulWidget {
  const RecoveryVisitPage({super.key, required this.caseId});

  final String caseId;

  @override
  State<RecoveryVisitPage> createState() => _RecoveryVisitPageState();
}

class _RecoveryVisitPageState extends State<RecoveryVisitPage> {
  int _step = 0;
  static const _stepTitles = ['Check-In', 'Customer', 'Photos', 'Documents', 'Sign-off'];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<RecoveryVisitProvider>().reset());
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
    final caseItem = context.watch<RecoveryCasesProvider>().selectedCase;
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        leading: IconButton(icon: const Icon(Icons.arrow_back), onPressed: _back),
        title: Text('Visit · ${caseItem?.customerName ?? ''}'),
      ),
      body: Column(
        children: [
          _StepIndicator(titles: _stepTitles, current: _step),
          Expanded(
            child: IndexedStack(
              index: _step,
              children: [
                _CheckInStep(onNext: _next),
                _CustomerPresenceStep(onNext: _next),
                _PhotosStep(onNext: _next),
                const _DocumentsStep(),
                _SignOffStep(caseId: widget.caseId),
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

class _CheckInStep extends StatefulWidget {
  const _CheckInStep({required this.onNext});

  final VoidCallback onNext;

  @override
  State<_CheckInStep> createState() => _CheckInStepState();
}

class _CheckInStepState extends State<_CheckInStep> {
  bool _capturing = false;

  Future<void> _checkIn() async {
    setState(() => _capturing = true);
    try {
      final permission = await Geolocator.checkPermission();
      var granted = permission;
      if (permission == LocationPermission.denied) {
        granted = await Geolocator.requestPermission();
      }
      if (granted == LocationPermission.denied || granted == LocationPermission.deniedForever) {
        if (mounted) AppSnackbar.danger(context, 'Location permission is required to check in');
        return;
      }
      if (!await Geolocator.isLocationServiceEnabled()) {
        if (mounted) AppSnackbar.danger(context, 'Please enable location services');
        return;
      }
      final position = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(accuracy: LocationAccuracy.high),
      );
      if (!mounted) return;
      context.read<RecoveryVisitProvider>().setCheckIn(position.latitude, position.longitude);
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not fetch current location');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final visit = context.watch<RecoveryVisitProvider>();
    final checkedIn = visit.checkInTime != null;

    return _StepScaffold(
      title: 'GPS Check-In',
      footer: AppButton(
        label: checkedIn ? 'Next: Customer' : 'Check in first',
        onPressed: checkedIn ? widget.onNext : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Check in at the customer\'s location to start the visit.',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.md),
          if (checkedIn)
            AppCard(
              padding: EdgeInsets.zero,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  ClipRRect(
                    borderRadius: const BorderRadius.vertical(top: Radius.circular(AppSpacing.radiusCard)),
                    child: SizedBox(
                      height: 180,
                      child: IgnorePointer(
                        child: FlutterMap(
                          options: MapOptions(
                            initialCenter: latlong.LatLng(visit.latitude!, visit.longitude!),
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
                                point: latlong.LatLng(visit.latitude!, visit.longitude!),
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
                  Padding(
                    padding: const EdgeInsets.all(AppSpacing.sm),
                    child: Text(
                      'Checked in at ${visit.checkInTime!.hour.toString().padLeft(2, '0')}:${visit.checkInTime!.minute.toString().padLeft(2, '0')}\n'
                      'Lat ${visit.latitude!.toStringAsFixed(6)}, Lng ${visit.longitude!.toStringAsFixed(6)}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                ],
              ),
            ),
          const SizedBox(height: AppSpacing.md),
          AppButton(
            label: checkedIn ? 'Re-check In' : 'Check In Now',
            icon: Icons.my_location_outlined,
            variant: AppButtonVariant.outline,
            isLoading: _capturing,
            onPressed: _checkIn,
          ),
        ],
      ),
    );
  }
}

class _CustomerPresenceStep extends StatefulWidget {
  const _CustomerPresenceStep({required this.onNext});

  final VoidCallback onNext;

  @override
  State<_CustomerPresenceStep> createState() => _CustomerPresenceStepState();
}

class _CustomerPresenceStepState extends State<_CustomerPresenceStep> {
  final _notesController = TextEditingController();

  @override
  void dispose() {
    _notesController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final visit = context.watch<RecoveryVisitProvider>();

    return _StepScaffold(
      title: 'Customer Presence & Notes',
      footer: AppButton(
        label: 'Next: Photos',
        onPressed: () {
          context.read<RecoveryVisitProvider>().setNotes(_notesController.text.trim());
          widget.onNext();
        },
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Was the customer present?', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    Expanded(
                      child: RadioListTile<bool>(
                        contentPadding: EdgeInsets.zero,
                        title: const Text('Present'),
                        value: true,
                        groupValue: visit.customerPresent,
                        activeColor: AppColors.secondary,
                        onChanged: (value) => context.read<RecoveryVisitProvider>().setCustomerPresent(value!),
                      ),
                    ),
                    Expanded(
                      child: RadioListTile<bool>(
                        contentPadding: EdgeInsets.zero,
                        title: const Text('Not Present'),
                        value: false,
                        groupValue: visit.customerPresent,
                        activeColor: AppColors.secondary,
                        onChanged: (value) => context.read<RecoveryVisitProvider>().setCustomerPresent(value!),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          AppTextField(label: 'Visit Notes', controller: _notesController, prefixIcon: Icons.notes_outlined),
        ],
      ),
    );
  }
}

class _PhotosStep extends StatefulWidget {
  const _PhotosStep({required this.onNext});

  final VoidCallback onNext;

  @override
  State<_PhotosStep> createState() => _PhotosStepState();
}

class _PhotosStepState extends State<_PhotosStep> {
  final _picker = ImagePicker();
  bool _capturing = false;

  Future<void> _capture() async {
    setState(() => _capturing = true);
    try {
      final photo = await _picker.pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) {
        context.read<RecoveryVisitProvider>().addPhoto(photo.path);
      }
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture photo');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final visit = context.watch<RecoveryVisitProvider>();

    return _StepScaffold(
      title: 'Photo Capture',
      footer: AppButton(label: 'Next: Documents', onPressed: widget.onNext),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Capture site or property photos as evidence for this visit.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.sm),
          if (visit.photoPaths.isNotEmpty)
            GridView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: visit.photoPaths.length,
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
                mainAxisSpacing: AppSpacing.sm,
                crossAxisSpacing: AppSpacing.sm,
              ),
              itemBuilder: (context, index) => ClipRRect(
                borderRadius: BorderRadius.circular(AppSpacing.radiusSm),
                child: Image.file(
                  File(visit.photoPaths[index]),
                  fit: BoxFit.cover,
                  errorBuilder: (context, error, stackTrace) => Container(
                    color: AppColors.surfaceMuted,
                    child: const Icon(Icons.image_outlined, color: AppColors.textSecondary),
                  ),
                ),
              ),
            ),
          const SizedBox(height: AppSpacing.md),
          AppButton(
            label: 'Capture Photo',
            icon: Icons.add_a_photo_outlined,
            variant: AppButtonVariant.outline,
            isLoading: _capturing,
            onPressed: _capture,
          ),
        ],
      ),
    );
  }
}

class _DocumentsStep extends StatefulWidget {
  const _DocumentsStep();

  @override
  State<_DocumentsStep> createState() => _DocumentsStepState();
}

class _DocumentsStepState extends State<_DocumentsStep> {
  static const _checklist = ['ID Proof', 'Address Proof', 'Loan Agreement Copy'];
  final _picker = ImagePicker();
  bool _capturing = false;

  Future<void> _capture(String type) async {
    setState(() => _capturing = true);
    try {
      final photo = await _picker.pickImage(source: ImageSource.camera, imageQuality: 70);
      if (photo != null && mounted) {
        context.read<RecoveryVisitProvider>().addDocument(photo.path);
      }
    } catch (_) {
      if (mounted) AppSnackbar.danger(context, 'Could not capture $type');
    } finally {
      if (mounted) setState(() => _capturing = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final visit = context.watch<RecoveryVisitProvider>();

    return _StepScaffold(
      title: 'Document Capture',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Capture supporting documents relevant to this recovery visit.',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: AppSpacing.sm),
          for (int i = 0; i < _checklist.length; i++)
            Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: AppCard(
                child: Row(
                  children: [
                    Container(
                      width: 44,
                      height: 44,
                      decoration: BoxDecoration(color: AppColors.surfaceMuted, borderRadius: BorderRadius.circular(AppSpacing.radiusSm)),
                      child: Icon(
                        i < visit.documentPaths.length ? Icons.check_circle : Icons.description_outlined,
                        color: i < visit.documentPaths.length ? AppColors.success : AppColors.textSecondary,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(child: Text(_checklist[i], style: Theme.of(context).textTheme.titleSmall)),
                    if (i < visit.documentPaths.length)
                      const Icon(Icons.check_circle, color: AppColors.success)
                    else
                      AppButton(
                        label: 'Capture',
                        size: AppButtonSize.small,
                        expanded: false,
                        isLoading: _capturing,
                        icon: Icons.camera_alt_outlined,
                        onPressed: () => _capture(_checklist[i]),
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

class _SignOffStep extends StatefulWidget {
  const _SignOffStep({required this.caseId});

  final String caseId;

  @override
  State<_SignOffStep> createState() => _SignOffStepState();
}

class _SignOffStepState extends State<_SignOffStep> {
  final _signature = SignatureController(penStrokeWidth: 3, penColor: AppColors.secondary);

  @override
  void dispose() {
    _signature.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final visit = context.read<RecoveryVisitProvider>();
    final caseItem = context.read<RecoveryCasesProvider>().selectedCase;
    if (_signature.isEmpty) {
      AppSnackbar.danger(context, 'Signature is required');
      return;
    }
    if (caseItem == null) {
      AppSnackbar.danger(context, 'Case details not loaded');
      return;
    }
    final bytes = await _signature.toPngBytes();
    if (bytes == null) return;
    final dir = await getApplicationDocumentsDirectory();
    final file = File('${dir.path}/recovery_sig_${const Uuid().v4()}.png');
    await file.writeAsBytes(bytes);
    if (!mounted) return;
    visit.setSignature(file.path);

    final record = await visit.submitVisit(widget.caseId, caseItem);
    if (!mounted) return;
    await context.read<RecoveryCasesProvider>().performAction(
          widget.caseId,
          RecoveryStatus.visited,
          'Visit Completed',
          note: 'Field visit completed and logged.',
        );
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(
        builder: (_) => RecoveryDocumentSuccessPage(
          title: 'Visit Submitted',
          message: 'Visit report for ${caseItem.customerName} has been saved.',
          pdfPath: record.pdfPath,
          shareText: 'Recovery visit report for ${caseItem.customerName}',
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final visit = context.watch<RecoveryVisitProvider>();

    return _StepScaffold(
      title: 'Sign-off',
      footer: AppButton(label: 'Submit Visit', isLoading: visit.isSubmitting, onPressed: _submit),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Customer / Officer Signature', style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: AppSpacing.sm),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Container(
                height: 160,
                decoration: BoxDecoration(border: Border.all(color: AppColors.border), borderRadius: BorderRadius.circular(AppSpacing.radiusMd)),
                child: Signature(controller: _signature, backgroundColor: Colors.white),
              ),
              TextButton(onPressed: _signature.clear, child: const Text('Clear')),
            ],
          ),
        ],
      ),
    );
  }
}
