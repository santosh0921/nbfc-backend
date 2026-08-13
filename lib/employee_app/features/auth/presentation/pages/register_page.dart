import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

/// Real, public employee self-registration — POST /employee/register. The
/// account is created pending admin approval (Approved=false server-side)
/// and cannot log in until an admin approves it and assigns an agency
/// (see internal/employee/register_handler.go, AdminApproveHandler).
class RegisterPage extends StatefulWidget {
  const RegisterPage({super.key});

  @override
  State<RegisterPage> createState() => _RegisterPageState();
}

class _RegisterPageState extends State<RegisterPage> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _passwordController = TextEditingController();
  final _confirmController = TextEditingController();
  final _cityController = TextEditingController();
  String _role = 'verification';
  bool _submitted = false;

  static const _roles = [
    (value: 'verification', label: 'Field Verification Officer'),
    (value: 'recovery', label: 'Collection Agent (Recovery)'),
    (value: 'supervisor', label: 'Supervisor'),
  ];

  @override
  void dispose() {
    _nameController.dispose();
    _passwordController.dispose();
    _confirmController.dispose();
    _cityController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final auth = context.read<AuthProvider>();
    final ok = await auth.register(
      name: _nameController.text.trim(),
      password: _passwordController.text,
      role: _role,
      branchCity: _cityController.text.trim(),
    );
    if (!mounted) return;
    if (ok) {
      setState(() => _submitted = true);
    } else if (auth.errorMessage != null) {
      AppSnackbar.danger(context, auth.errorMessage!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final isLoading = auth.status == AuthStatus.authenticating;

    if (_submitted) {
      return AppScaffold(
        showAppBar: false,
        padding: const EdgeInsets.all(AppSpacing.lg),
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 72,
                height: 72,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: AppColors.secondary, borderRadius: BorderRadius.circular(AppSpacing.radiusLg)),
                child: const Icon(Icons.hourglass_top_rounded, color: AppColors.primary, size: 34),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('Account created', style: Theme.of(context).textTheme.headlineSmall, textAlign: TextAlign.center),
              const SizedBox(height: 8),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg),
                child: Text(
                  "Your account is pending admin approval. You'll be able to log in once an admin approves it and assigns you to an agency.",
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
                ),
              ),
              const SizedBox(height: AppSpacing.xl),
              AppButton(label: 'Back to Sign In', onPressed: () => context.goNamed(RouteNames.login)),
            ],
          ),
        ),
      );
    }

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
                children: [
                  const SizedBox(height: AppSpacing.xl),
                  Text('Create Employee Account', style: Theme.of(context).textTheme.headlineSmall),
                  const SizedBox(height: 4),
                  Text(
                    "Sign up, then wait for your admin to approve access",
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: AppColors.textSecondary),
                  ),
                  const SizedBox(height: AppSpacing.xl),
                  AppTextField(
                    label: 'Full Name',
                    hint: 'e.g. Priya Deshmukh',
                    controller: _nameController,
                    prefixIcon: Icons.badge_outlined,
                    keyboardType: TextInputType.name,
                    textInputAction: TextInputAction.next,
                    validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your full name' : null,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  AppTextField(
                    label: 'Password',
                    controller: _passwordController,
                    prefixIcon: Icons.lock_outline,
                    obscureText: true,
                    textInputAction: TextInputAction.next,
                    validator: (v) => (v == null || v.length < 6) ? 'At least 6 characters' : null,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  AppTextField(
                    label: 'Confirm Password',
                    controller: _confirmController,
                    prefixIcon: Icons.lock_outline,
                    obscureText: true,
                    textInputAction: TextInputAction.next,
                    validator: (v) => v != _passwordController.text ? 'Passwords do not match' : null,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  Text('Role', style: Theme.of(context).textTheme.labelLarge),
                  const SizedBox(height: AppSpacing.xs),
                  DropdownButtonFormField<String>(
                    value: _role,
                    decoration: const InputDecoration(prefixIcon: Icon(Icons.work_outline), border: OutlineInputBorder()),
                    items: [for (final r in _roles) DropdownMenuItem(value: r.value, child: Text(r.label))],
                    onChanged: (v) => setState(() => _role = v ?? _role),
                  ),
                  const SizedBox(height: AppSpacing.md),
                  AppTextField(
                    label: 'Branch City',
                    hint: 'e.g. Mumbai',
                    controller: _cityController,
                    prefixIcon: Icons.location_city_outlined,
                    textInputAction: TextInputAction.done,
                    validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your branch city' : null,
                  ),
                  const SizedBox(height: AppSpacing.xl),
                  AppButton(label: 'Create Account', isLoading: isLoading, onPressed: _submit),
                  const SizedBox(height: AppSpacing.md),
                  Center(
                    child: TextButton(
                      onPressed: isLoading ? null : () => context.goNamed(RouteNames.login),
                      child: const Text('Already have an account? Sign In'),
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
