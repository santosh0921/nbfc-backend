import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

/// Local-only "app lock" gate shown when a still-valid JWT session was
/// restored on app start and this employee had previously set an MPIN on
/// this device (see [AuthProvider.needsMpinUnlock] / `MpinService`'s doc
/// comment). No backend call happens here — a correct MPIN simply clears
/// the local gate and lets the already-authenticated session through, the
/// same way re-entering a PIN unlocks an already-logged-in banking app.
class MpinUnlockPage extends StatefulWidget {
  const MpinUnlockPage({super.key});

  @override
  State<MpinUnlockPage> createState() => _MpinUnlockPageState();
}

class _MpinUnlockPageState extends State<MpinUnlockPage> {
  final _formKey = GlobalKey<FormState>();
  final _mpinController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _mpinController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    final auth = context.read<AuthProvider>();
    final ok = await auth.verifyMpin(_mpinController.text.trim());
    if (!mounted) return;
    setState(() => _submitting = false);
    if (!ok) {
      _mpinController.clear();
      AppSnackbar.danger(context, 'Incorrect MPIN. Try again.');
    }
    // On success, needsMpinUnlock flips to false and the router's
    // refreshListenable takes the employee straight to their dashboard.
  }

  Future<void> _useFullLoginInstead() async {
    await context.read<AuthProvider>().logout();
  }

  @override
  Widget build(BuildContext context) {
    final employeeName = context.watch<AuthProvider>().employee?.name;
    return PopScope(
      // Back from the unlock gate has nowhere sensible to go but out of
      // the app — there is no "previous step" to return to.
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        SystemNavigator.pop();
      },
      child: AppScaffold(
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
                    Text('Enter your MPIN', style: Theme.of(context).textTheme.headlineSmall),
                    const SizedBox(height: 4),
                    Text(
                      employeeName != null ? 'Welcome back, $employeeName' : 'Unlock to continue',
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    AppTextField(
                      label: 'MPIN',
                      controller: _mpinController,
                      prefixIcon: Icons.pin_outlined,
                      obscureText: true,
                      keyboardType: TextInputType.number,
                      textInputAction: TextInputAction.done,
                      validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your MPIN' : null,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    AppButton(label: 'Unlock', isLoading: _submitting, onPressed: _submit),
                    const SizedBox(height: AppSpacing.sm),
                    Center(
                      child: TextButton(
                        onPressed: _submitting ? null : _useFullLoginInstead,
                        child: const Text('Forgot MPIN? Sign in with name & password'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
