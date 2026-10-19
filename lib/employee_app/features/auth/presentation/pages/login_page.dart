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
import '../../domain/entities/employee_entity.dart';
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

  @override
  void initState() {
    super.initState();
    final auth = context.read<AuthProvider>();
    if (auth.rememberedIdentifier != null) {
      _nameController.text = auth.rememberedIdentifier!;
      _rememberMe = true;
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  // Which dashboard route to land on is derived from the employee's own
  // role (returned by the login response) — not asked of the user, since
  // every employee has exactly one role/module. See
  // AppRouter._homePathFor for the equivalent logic the redirect guard
  // uses when a session is restored rather than freshly logged in.
  void _afterAuthenticated(AuthProvider auth) {
    if (auth.needsFirstTimeOnboarding) {
      context.goNamed(RouteNames.otpVerify);
    } else if (auth.employee?.role == EmployeeRole.collectionAgent) {
      context.goNamed(RouteNames.recoveryDashboard);
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
  // share the password "onefin123".
  void _fillDemoAccount(String name) {
    setState(() {
      _nameController.text = name;
      _passwordController.text = 'onefin123';
    });
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final isLoading = auth.status == AuthStatus.authenticating;

    return PopScope(
      // This login page is the employee app's own initial route, with
      // nothing left to pop inside its own (nested) router — the employee
      // app is a second MaterialApp.router mounted inside the customer
      // app (see EmployeeGateScreen/OnefinEmployeeApp), and that inner
      // router's default back-button handling was quitting the whole app
      // outright instead of falling back to the customer app's own
      // role-selection screen underneath it. Intercepting the pop here
      // and driving the OUTER (root) Navigator instead is what actually
      // takes the user back to Verification/Recovery role selection.
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        Navigator.of(context, rootNavigator: true).maybePop();
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
                  children: [
                    const SizedBox(height: AppSpacing.xl),
                    Container(
                      width: 72,
                      height: 72,
                      alignment: Alignment.center,
                      decoration: BoxDecoration(
                        color: AppColors.secondary,
                        borderRadius:
                            BorderRadius.circular(AppSpacing.radiusLg),
                      ),
                      child: const Icon(Icons.shield_outlined,
                          color: AppColors.primary, size: 34),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    Text('ONEFIN Employee',
                        style: Theme.of(context).textTheme.headlineSmall),
                    const SizedBox(height: 4),
                    Text(
                      'Sign in to manage cases, visits & collections',
                      style: Theme.of(context)
                          .textTheme
                          .bodyMedium
                          ?.copyWith(color: AppColors.textSecondary),
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
                      validator: (v) => (v == null || v.trim().isEmpty)
                          ? 'Enter your full name'
                          : null,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    AppTextField(
                      label: 'Password',
                      hint: 'Enter your password',
                      controller: _passwordController,
                      prefixIcon: Icons.lock_outline,
                      obscureText: true,
                      textInputAction: TextInputAction.done,
                      autofillHints: const [AutofillHints.password],
                      validator: (v) => (v == null || v.length < 4)
                          ? 'Enter a valid password'
                          : null,
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Expanded(
                          child: InkWell(
                            onTap: () =>
                                setState(() => _rememberMe = !_rememberMe),
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Checkbox(
                                    value: _rememberMe,
                                    onChanged: (v) => setState(
                                        () => _rememberMe = v ?? false)),
                                const Flexible(
                                    child: Text('Remember me',
                                        overflow: TextOverflow.ellipsis)),
                              ],
                            ),
                          ),
                        ),
                        TextButton(
                          onPressed: () =>
                              context.pushNamed(RouteNames.forgotPassword),
                          child: const Text('Forgot Password?'),
                        ),
                      ],
                    ),
                    const SizedBox(height: AppSpacing.md),
                    AppButton(
                        label: 'Sign In',
                        isLoading: isLoading,
                        onPressed: _submit),
                    const SizedBox(height: AppSpacing.sm),
                    Center(
                      child: TextButton(
                        onPressed: isLoading
                            ? null
                            : () => context.pushNamed(RouteNames.register),
                        child: const Text('New employee? Create an account'),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.xl),
                    // Two seeded demo accounts (internal/seed/seed.go), one
                    // per role — a single "use demo account" shortcut can't
                    // guess which module the tester wants to see now that
                    // login no longer starts from a role choice.
                    Center(
                      child: Wrap(
                        alignment: WrapAlignment.center,
                        spacing: 4,
                        children: [
                          TextButton(
                            onPressed: isLoading
                                ? null
                                : () => _fillDemoAccount('Priya Deshmukh'),
                            style: TextButton.styleFrom(
                                foregroundColor: AppColors.textSecondary),
                            child: const Text('Demo Verification account',
                                style: TextStyle(
                                    decoration: TextDecoration.underline)),
                          ),
                          TextButton(
                            onPressed: isLoading
                                ? null
                                : () => _fillDemoAccount('Anita Rao'),
                            style: TextButton.styleFrom(
                                foregroundColor: AppColors.textSecondary),
                            child: const Text('Demo Recovery account',
                                style: TextStyle(
                                    decoration: TextDecoration.underline)),
                          ),
                        ],
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
