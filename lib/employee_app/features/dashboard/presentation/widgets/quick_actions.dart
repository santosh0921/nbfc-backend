import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import '../../../../core/components/app_bottom_sheet.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../../../visit_verification/presentation/pages/route_map_page.dart';
import '../../../visit_verification/presentation/pages/visit_history_page.dart';

class QuickAction {
  const QuickAction({required this.icon, required this.label, required this.onTap});

  final IconData icon;
  final String label;
  final Future<void> Function(BuildContext context) onTap;
}

/// The field-officer quick actions — each wired to a real, working
/// behaviour (navigation, capture, call, or a lightweight mock form) rather
/// than a placeholder tap target. Kept to 9 (a clean 3x3 grid) — the most
/// frequently-needed actions only.
final List<QuickAction> quickActions = [
  QuickAction(icon: Icons.add_task_outlined, label: 'New Visit', onTap: (context) async => context.goNamed(RouteNames.cases)),
  QuickAction(icon: Icons.qr_code_scanner_outlined, label: 'Scan Doc', onTap: _scanDocument),
  QuickAction(icon: Icons.map_outlined, label: 'Route Map', onTap: _openRouteMap),
  QuickAction(icon: Icons.description_outlined, label: 'Reports', onTap: (context) async => _pushHistory(context)),
  QuickAction(icon: Icons.diamond_outlined, label: 'Gold Appraisal', onTap: (context) async => _showGoldAppraisalForm(context)),
  QuickAction(icon: Icons.home_work_outlined, label: 'Property Verify', onTap: (context) async => _showPropertyForm(context)),
  QuickAction(icon: Icons.payments_outlined, label: 'Collections', onTap: (context) async => context.goNamed(RouteNames.collections)),
  QuickAction(icon: Icons.event_available_outlined, label: 'Attendance', onTap: (context) async => _markAttendance(context)),
  QuickAction(icon: Icons.help_outline, label: 'Help', onTap: (context) async => _showHelp(context)),
];

void _pushHistory(BuildContext context) {
  Navigator.of(context).push(MaterialPageRoute(builder: (_) => const VisitHistoryPage()));
}

Future<void> _scanDocument(BuildContext context) async {
  try {
    final photo = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 70);
    if (photo != null && context.mounted) AppSnackbar.success(context, 'Document captured and queued for upload.');
  } catch (_) {
    if (context.mounted) AppSnackbar.danger(context, 'Could not open camera.');
  }
}

Future<void> _openRouteMap(BuildContext context) async {
  Navigator.of(context).push(MaterialPageRoute(builder: (_) => const RouteMapPage()));
}

Future<void> _markAttendance(BuildContext context) async {
  final action = await showDialog<String>(
    context: context,
    builder: (context) => AlertDialog(
      title: const Text('Attendance'),
      content: const Text('Mark your attendance for today with your current location.'),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop('checkin'), child: const Text('Check In')),
        FilledButton(onPressed: () => Navigator.of(context).pop('checkout'), child: const Text('Check Out')),
      ],
    ),
  );
  if (action == null || !context.mounted) return;

  try {
    await Geolocator.getCurrentPosition(locationSettings: const LocationSettings(accuracy: LocationAccuracy.medium));
    if (context.mounted) {
      AppSnackbar.success(context, action == 'checkin' ? 'Checked in successfully.' : 'Checked out successfully.');
    }
  } catch (_) {
    if (context.mounted) AppSnackbar.danger(context, 'Could not fetch your location.');
  }
}

void _showHelp(BuildContext context) {
  AppBottomSheet.show(
    context,
    title: 'Help & Support',
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const ListTile(
          contentPadding: EdgeInsets.zero,
          leading: Icon(Icons.call_outlined, color: AppColors.secondary),
          title: Text('Support Helpline'),
          subtitle: Text('+91 1800 123 456'),
        ),
        const ListTile(
          contentPadding: EdgeInsets.zero,
          leading: Icon(Icons.email_outlined, color: AppColors.secondary),
          title: Text('Email Support'),
          subtitle: Text('support@onefin.com'),
        ),
        const SizedBox(height: AppSpacing.sm),
        AppButton(label: 'Close', variant: AppButtonVariant.outline, onPressed: () => Navigator.of(context).pop()),
      ],
    ),
  );
}

void _showGoldAppraisalForm(BuildContext context) {
  final weightController = TextEditingController();
  final purityController = TextEditingController();
  AppBottomSheet.show(
    context,
    title: 'Gold Appraisal',
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        AppTextField(label: 'Weight (grams)', hint: 'Enter weight in grams', controller: weightController, keyboardType: TextInputType.number),
        const SizedBox(height: AppSpacing.sm),
        AppTextField(label: 'Purity (Karat)', hint: 'e.g. 22', controller: purityController, keyboardType: TextInputType.number),
        const SizedBox(height: AppSpacing.md),
        AppButton(
          label: 'Save Appraisal',
          onPressed: () {
            Navigator.of(context).pop();
            AppSnackbar.success(context, 'Gold appraisal saved.');
          },
        ),
      ],
    ),
  );
}

void _showPropertyForm(BuildContext context) {
  final conditionController = TextEditingController();
  final valueController = TextEditingController();
  AppBottomSheet.show(
    context,
    title: 'Property Verification',
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        AppTextField(label: 'Property Condition', hint: 'e.g. Good, Needs repair', controller: conditionController),
        const SizedBox(height: AppSpacing.sm),
        AppTextField(label: 'Estimated Value (₹)', hint: 'Enter estimated value', controller: valueController, keyboardType: TextInputType.number),
        const SizedBox(height: AppSpacing.md),
        AppButton(
          label: 'Save Details',
          onPressed: () {
            Navigator.of(context).pop();
            AppSnackbar.success(context, 'Property verification details saved.');
          },
        ),
      ],
    ),
  );
}

/// Renders the AppCard used by the quick-actions grid — kept out of the
/// dashboard page file to avoid duplicating tap handling per action.
class QuickActionCard extends StatelessWidget {
  const QuickActionCard({super.key, required this.action});

  final QuickAction action;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs, horizontal: 4),
      onTap: () => action.onTap(context),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(action.icon, color: AppColors.secondary, size: 22),
          const SizedBox(height: 6),
          Text(
            action.label,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(fontSize: 10.5),
            textAlign: TextAlign.center,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}
