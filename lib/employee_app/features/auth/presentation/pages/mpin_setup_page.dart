import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

/// Final step of first-time, per-device onboarding: optionally set a short
/// numeric MPIN used purely as a local "app lock" quick-unlock on this
/// device for future app opens (see [MpinService]'s doc comment — it never
/// touches the backend password). Skippable, since it is a convenience
/// feature, not the actual account security (name+password already is).
class MpinSetupPage extends StatefulWidget {
  const MpinSetupPage({super.key});

  @override
  State<MpinSetupPage> createState() => _MpinSetupPageState();
}

class _MpinSetupPageState extends State<MpinSetupPage> {
  final _formKey = GlobalKey<FormState>();
  final _mpinController = TextEditingController();
  final _confirmController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _mpinController.dispose();
    _confirmController.dispose();
    super.dispose();
  }

  Future<void> _finish({required bool setMpin}) async {
    final auth = context.read<AuthProvider>();
    if (setMpin) {
      if (!_formKey.currentState!.validate()) return;
      if (_mpinController.text != _confirmController.text) {
        AppSnackbar.danger(context, 'MPINs do not match.');
        return;
      }
      setState(() => _submitting = true);
      await auth.setMpin(_mpinController.text);
    }
    await auth.completeOnboarding();
    // The router's redirect listener picks up the flag change and takes
    // the employee to the right dashboard automatically — no explicit
    // navigation call needed here.
    if (mounted && _submitting) setState(() => _submitting = false);
  }

  @override
  Widget build(BuildContext context) {
    return AppScaffold(
      showAppBar: false,
      padding: const EdgeInsets.all(AppSpacing.lg),
      body: Center(
        child: SingleChildScrollView(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Form(
              key: _formKey,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: 72,
                    height: 72,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: AppColors.secondary,
                      borderRadius: BorderRadius.circular(AppSpacing.radiusLg),
                    ),
                    child: const Icon(Icons.lock_outline, color: AppColors.primary, size: 34),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  Text('Set your MPIN', style: Theme.of(context).textTheme.headlineSmall),
                  const SizedBox(height: 4),
                  Text(
                    'Add a 4-6 digit PIN for quick access on this device next time. '
                    'You can skip this and use your name and password instead.',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  AppTextField(
                    label: 'New MPIN',
                    hint: '4-6 digits',
                    controller: _mpinController,
                    prefixIcon: Icons.pin_outlined,
                    obscureText: true,
                    keyboardType: TextInputType.number,
                    textInputAction: TextInputAction.next,
                    validator: (v) {
                      final value = v?.trim() ?? '';
                      if (value.length < 4 || value.length > 6) return 'Enter 4-6 digits';
                      if (!RegExp(r'^\d+$').hasMatch(value)) return 'Digits only';
                      return null;
                    },
                  ),
                  const SizedBox(height: AppSpacing.md),
                  AppTextField(
                    label: 'Confirm MPIN',
                    hint: 'Re-enter your MPIN',
                    controller: _confirmController,
                    prefixIcon: Icons.pin_outlined,
                    obscureText: true,
                    keyboardType: TextInputType.number,
                    textInputAction: TextInputAction.done,
                    validator: (v) => (v == null || v.trim().isEmpty) ? 'Confirm your MPIN' : null,
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  AppButton(label: 'Set MPIN', isLoading: _submitting, onPressed: () => _finish(setMpin: true)),
                  const SizedBox(height: AppSpacing.sm),
                  Center(
                    child: TextButton(
                      onPressed: _submitting ? null : () => _finish(setMpin: false),
                      child: const Text('Skip for now'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
