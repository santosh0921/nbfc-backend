import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/app_snackbar.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/constants/employee_app_module.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../../core/widgets/app_text_field.dart';
import '../providers/auth_provider.dart';

class LoginPage extends StatefulWidget {
  const LoginPage({super.key});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _rememberMe = false;
  bool _biometricAvailable = false;

  @override
  void initState() {
    super.initState();
    final auth = context.read<AuthProvider>();
    if (auth.rememberedIdentifier != null) {
      _nameController.text = auth.rememberedIdentifier!;
      _rememberMe = true;
    }
    auth.canUseBiometrics.then((value) {
      if (mounted) setState(() => _biometricAvailable = value);
    });
  }

  @override
  void dispose() {
    _nameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _afterAuthenticated(AuthProvider auth) {
    if (auth.needsFirstTimeOnboarding) {
      context.goNamed(RouteNames.otpVerify);
    } else {
      context.goNamed(RouteNames.dashboard);
    }
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final auth = context.read<AuthProvider>();
    await auth.login(
      name: _nameController.text.trim(),
      password: _passwordController.text,
      remember: _rememberMe,
    );
    if (!mounted) return;
    if (auth.status == AuthStatus.authenticated) {
      _afterAuthenticated(auth);
    } else if (auth.errorMessage != null) {
      AppSnackbar.danger(context, auth.errorMessage!);
    }
  }

  // Seeded demo employees from the backend (internal/seed/seed.go) — all
  // share the password "onefin123". Login is name-based now, so these are
  // the seeded employees' real names, not their old internal codes.
  void _fillDemoAccount(EmployeeAppModule module) {
    final name = module == EmployeeAppModule.recovery ? 'Anita Rao' : 'Priya Deshmukh';
    setState(() {
      _nameController.text = name;
      _passwordController.text = 'onefin123';
    });
  }

  Future<void> _submitBiometric() async {
    final auth = context.read<AuthProvider>();
    await auth.loginWithBiometrics();
    if (!mounted) return;
    if (auth.status == AuthStatus.authenticated) {
      _afterAuthenticated(auth);
    } else if (auth.errorMessage != null) {
      AppSnackbar.danger(context, auth.errorMessage!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final isLoading = auth.status == AuthStatus.authenticating;

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
                  Container(
                    width: 72,
                    height: 72,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: AppColors.secondary,
                      borderRadius: BorderRadius.circular(AppSpacing.radiusLg),
                    ),
                    child: const Icon(Icons.shield_outlined, color: AppColors.primary, size: 34),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  Text('ONEFIN Employee', style: Theme.of(context).textTheme.headlineSmall),
                  const SizedBox(height: 4),
                  Text(
                    'Sign in to manage cases, visits & collections',
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
                    autofillHints: const [AutofillHints.name],
                    validator: (v) => (v == null || v.trim().isEmpty) ? 'Enter your full name' : null,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  AppTextField(
                    label: 'Password',
                    controller: _passwordController,
                    prefixIcon: Icons.lock_outline,
                    obscureText: true,
                    textInputAction: TextInputAction.done,
                    autofillHints: const [AutofillHints.password],
                    validator: (v) => (v == null || v.length < 4) ? 'Enter a valid password' : null,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Expanded(
                        child: InkWell(
                          onTap: () => setState(() => _rememberMe = !_rememberMe),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Checkbox(value: _rememberMe, onChanged: (v) => setState(() => _rememberMe = v ?? false)),
                              const Flexible(child: Text('Remember me', overflow: TextOverflow.ellipsis)),
                            ],
                          ),
                        ),
                      ),
                      TextButton(
                        onPressed: () => context.pushNamed(RouteNames.forgotPassword),
                        child: const Text('Forgot Password?'),
                      ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.md),
                  AppButton(label: 'Sign In', isLoading: isLoading, onPressed: _submit),
                  if (_biometricAvailable) ...[
                    const SizedBox(height: AppSpacing.md),
                    Row(
                      children: [
                        Expanded(child: Divider(color: AppColors.divider)),
                        Padding(
                          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm),
                          child: Text('or', style: Theme.of(context).textTheme.bodySmall),
                        ),
                        Expanded(child: Divider(color: AppColors.divider)),
                      ],
                    ),
                    const SizedBox(height: AppSpacing.md),
                    AppButton(
                      label: 'Sign in with Biometrics',
                      variant: AppButtonVariant.outline,
                      icon: Icons.fingerprint,
                      onPressed: isLoading ? null : _submitBiometric,
                    ),
                  ],
                  const SizedBox(height: AppSpacing.xl),
                  Center(
                    child: TextButton(
                      onPressed: isLoading ? null : () => _fillDemoAccount(auth.module),
                      style: TextButton.styleFrom(foregroundColor: AppColors.textSecondary),
                      child: Text(
                        auth.module == EmployeeAppModule.recovery
                            ? 'Use demo Recovery account'
                            : 'Use demo Verification account',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                              color: AppColors.textSecondary,
                              decoration: TextDecoration.underline,
                            ),
                      ),
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
