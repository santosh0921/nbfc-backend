import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

class ForgotPasswordPage extends StatefulWidget {
  const ForgotPasswordPage({super.key});

  @override
  State<ForgotPasswordPage> createState() => _ForgotPasswordPageState();
}

class _ForgotPasswordPageState extends State<ForgotPasswordPage> {
  final _formKey = GlobalKey<FormState>();
  final _identifierController = TextEditingController();
  bool _isSubmitting = false;

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isSubmitting = true);
    final failure = await context.read<AuthProvider>().forgotPassword(_identifierController.text.trim());
    if (!mounted) return;
    setState(() => _isSubmitting = false);
    if (failure == null) {
      AppSnackbar.success(context, 'If an account exists for this ID, reset instructions would be sent to the registered contact.');
      Navigator.of(context).pop();
    } else {
      AppSnackbar.danger(context, failure.message);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AppScaffold(
      title: 'Forgot Password',
      padding: const EdgeInsets.all(AppSpacing.lg),
      body: Form(
        key: _formKey,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const SizedBox(height: AppSpacing.md),
            Text(
              "Enter your employee ID or email and we'll send reset instructions.",
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: AppSpacing.lg),
            AppTextField(
              label: 'Employee ID / Email',
              hint: 'Enter your employee ID or email',
              controller: _identifierController,
              prefixIcon: Icons.badge_outlined,
              validator: (v) => (v == null || v.trim().isEmpty) ? 'This field is required' : null,
            ),
            const SizedBox(height: AppSpacing.lg),
            AppButton(label: 'Send Reset Link', isLoading: _isSubmitting, onPressed: _submit),
          ],
        ),
      ),
    );
  }
}
