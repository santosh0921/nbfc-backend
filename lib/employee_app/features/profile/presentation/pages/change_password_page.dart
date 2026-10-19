import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/di/service_locator.dart';
import '../../../../core/network/api_client.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';

class ChangePasswordPage extends StatefulWidget {
  const ChangePasswordPage({super.key});

  @override
  State<ChangePasswordPage> createState() => _ChangePasswordPageState();
}

class _ChangePasswordPageState extends State<ChangePasswordPage> {
  final _formKey = GlobalKey<FormState>();
  final _currentController = TextEditingController();
  final _newController = TextEditingController();
  final _confirmController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _currentController.dispose();
    _newController.dispose();
    _confirmController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    setState(() => _submitting = true);
    try {
      final apiClient = ServiceLocator.instance.get<ApiClient>();
      await apiClient.dio.post('/employee/change-password', data: {
        'currentPassword': _currentController.text,
        'newPassword': _newController.text,
      });
      if (!mounted) return;
      AppSnackbar.success(context, 'Password updated successfully.');
      Navigator.of(context).pop();
    } on DioException catch (e) {
      final message = (e.response?.data is Map) ? (e.response?.data['message'] as String?) : null;
      if (mounted) AppSnackbar.danger(context, message ?? 'Could not update your password.');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AppScaffold(
      title: 'Change Password',
      leading: const BackButton(),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.md),
          children: [
            AppTextField(
              label: 'Current Password',
              controller: _currentController,
              obscureText: true,
              validator: (v) => (v == null || v.isEmpty) ? 'Enter your current password' : null,
            ),
            const SizedBox(height: AppSpacing.md),
            AppTextField(
              label: 'New Password',
              controller: _newController,
              obscureText: true,
              validator: (v) => (v == null || v.length < 6) ? 'New password must be at least 6 characters' : null,
            ),
            const SizedBox(height: AppSpacing.md),
            AppTextField(
              label: 'Confirm New Password',
              controller: _confirmController,
              obscureText: true,
              validator: (v) => (v != _newController.text) ? 'Passwords do not match' : null,
            ),
            const SizedBox(height: AppSpacing.lg),
            AppButton(
              label: _submitting ? 'Updating...' : 'Update Password',
              onPressed: _submitting ? null : _submit,
            ),
          ],
        ),
      ),
    );
  }
}
