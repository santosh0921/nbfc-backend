import 'package:flutter/material.dart';
import '../../../../core/constants/employee_app_module.dart';
import '../../../../core/error/failure.dart';
import '../../../../core/storage/local_storage_service.dart';
import '../../../../core/storage/mpin_service.dart';
import '../../../../core/usecase/usecase.dart';
import '../../domain/entities/employee_entity.dart';
import '../../domain/repositories/auth_repository.dart';
import '../../domain/usecases/login_usecase.dart';

enum AuthStatus { initial, authenticating, authenticated, unauthenticated, error }

class AuthProvider extends ChangeNotifier {
  AuthProvider({
    required LoginUseCase loginUseCase,
    required RegisterUseCase registerUseCase,
    required LogoutUseCase logoutUseCase,
    required ForgotPasswordUseCase forgotPasswordUseCase,
    required AuthRepository authRepository,
    required LocalStorageService localStorage,
    required MpinService mpinService,
    this.module = EmployeeAppModule.verification,
  })  : _loginUseCase = loginUseCase,
        _registerUseCase = registerUseCase,
        _logoutUseCase = logoutUseCase,
        _forgotPasswordUseCase = forgotPasswordUseCase,
        _authRepository = authRepository,
        _localStorage = localStorage,
        _mpinService = mpinService {
    _restoreSession();
  }

  final LoginUseCase _loginUseCase;
  final RegisterUseCase _registerUseCase;
  final LogoutUseCase _logoutUseCase;
  final ForgotPasswordUseCase _forgotPasswordUseCase;
  final AuthRepository _authRepository;
  final LocalStorageService _localStorage;
  final MpinService _mpinService;

  /// Retained only so existing call sites/DI wiring compile unchanged.
  /// No longer used to gate or reject a login: which dashboard an employee
  /// lands on is derived from their actual [EmployeeEntity.role] (see
  /// `AppRouter._homePathFor`), not a module the user had to guess before
  /// authenticating. The old flow rejected a correct password if the
  /// pre-picked module didn't match the account's real role — confusing,
  /// and redundant once every employee's role already determines their one
  /// module.
  @Deprecated('No longer used for login gating — role from the backend is authoritative.')
  final EmployeeAppModule module;

  static const _kRememberedIdKey = 'remembered_employee_id';
  static const _kProfileCompletedPrefix = 'profile_completed_';
  static const _kOnboardingCompletedPrefix = 'employee_onboarding_completed_';

  AuthStatus status = AuthStatus.initial;
  EmployeeEntity? employee;
  String? errorMessage;
  bool rememberMe = false;
  String? rememberedIdentifier;

  /// True once an employee is authenticated but hasn't completed their
  /// first-time profile setup yet — the router redirects here before
  /// letting them reach the dashboard.
  bool needsProfileSetup = false;

  /// True right after a fresh Name+Password login on a device where this
  /// employee (by employee code) has never completed the one-time
  /// "verify it's you" (mock OTP) + "set your MPIN" onboarding sequence.
  /// Not set when a session is silently restored from an existing JWT.
  bool needsFirstTimeOnboarding = false;

  /// True when a still-valid JWT session was restored on app start for an
  /// employee who previously set a local MPIN on this device — the router
  /// shows an MPIN-entry "app lock" screen before admitting them, purely as
  /// a local quick-unlock gate in front of the already-authenticated
  /// session (no backend call involved). Cleared once the MPIN is verified.
  bool needsMpinUnlock = false;

  Future<void> _restoreSession() async {
    rememberedIdentifier = await _localStorage.getString(_kRememberedIdKey);
    rememberMe = rememberedIdentifier != null;
    final session = await _authRepository.currentSession();
    if (session != null) {
      employee = session;
      status = AuthStatus.authenticated;
      await _refreshProfileSetupFlag();
      await _refreshOnboardingFlag();
      // Only gate a *restored* session behind the MPIN lock — a session
      // that was just created via the login form is handled by [login]
      // itself (it goes to onboarding first if needed, otherwise straight
      // to the dashboard; there is nothing to "unlock" yet).
      needsMpinUnlock = !needsFirstTimeOnboarding && await _mpinService.hasMpin(session.employeeCode);
    } else {
      status = AuthStatus.unauthenticated;
    }
    notifyListeners();
  }

  Future<void> _refreshOnboardingFlag() async {
    if (employee == null) return;
    final completed = await _localStorage.getBool('$_kOnboardingCompletedPrefix${employee!.employeeCode}');
    needsFirstTimeOnboarding = completed != true;
  }

  /// Marks the one-time mock-OTP + MPIN-setup sequence as done for the
  /// current employee on this device (called after the MPIN-setup step,
  /// whether the employee actually set an MPIN or chose to skip it).
  Future<void> completeOnboarding() async {
    if (employee == null) return;
    await _localStorage.setBool('$_kOnboardingCompletedPrefix${employee!.employeeCode}', true);
    needsFirstTimeOnboarding = false;
    notifyListeners();
  }

  /// Sets (or replaces) the local MPIN for the current employee. See
  /// [MpinService]'s doc comment for why this is a local-only app lock and
  /// not a new backend auth mechanism.
  Future<void> setMpin(String mpin) async {
    if (employee == null) return;
    await _mpinService.setMpin(employee!.employeeCode, mpin);
  }

  Future<bool> verifyMpin(String mpin) async {
    if (employee == null) return false;
    final ok = await _mpinService.verifyMpin(employee!.employeeCode, mpin);
    if (ok) {
      needsMpinUnlock = false;
      notifyListeners();
    }
    return ok;
  }

  Future<void> _refreshProfileSetupFlag() async {
    if (employee == null) return;
    final completed = await _localStorage.getBool('$_kProfileCompletedPrefix${employee!.id}');
    needsProfileSetup = completed != true;
  }

  Future<void> completeProfileSetup() async {
    if (employee == null) return;
    await _localStorage.setBool('$_kProfileCompletedPrefix${employee!.id}', true);
    needsProfileSetup = false;
    notifyListeners();
  }

  Future<void> login({required String name, required String password, bool remember = false}) async {
    status = AuthStatus.authenticating;
    errorMessage = null;
    notifyListeners();

    // `module` is no longer known before login — the backend no longer
    // consults it either (see LoginHandler's doc comment), so a fixed
    // placeholder is sent here purely for API-shape compatibility.
    final result = await _loginUseCase(LoginParams(name: name, password: password, module: EmployeeAppModule.verification.name));
    await result.when(
      success: (data) async {
        employee = data;
        status = AuthStatus.authenticated;
        rememberMe = remember;
        if (remember) {
          await _localStorage.setString(_kRememberedIdKey, name);
        } else {
          await _localStorage.remove(_kRememberedIdKey);
        }
        // A session freshly created by submitting the login form is never
        // MPIN-gated (that gate only guards a *restored* JWT session) — it
        // only needs the one-time onboarding sequence if this employee
        // hasn't completed it yet on this device.
        needsMpinUnlock = false;
      },
      failure: (f) async {
        status = AuthStatus.error;
        errorMessage = f.message;
      },
    );
    if (status == AuthStatus.authenticated) {
      await _refreshProfileSetupFlag();
      await _refreshOnboardingFlag();
    }
    notifyListeners();
  }

  /// True right after a successful registration submit — the UI uses this
  /// to show a "pending admin approval" confirmation instead of trying to
  /// treat it like a login (there is no session to start: the account
  /// isn't approved yet).
  bool justRegistered = false;

  Future<bool> register({required String name, required String password, required String role, required String branchCity}) async {
    status = AuthStatus.authenticating;
    errorMessage = null;
    justRegistered = false;
    notifyListeners();

    final result = await _registerUseCase(RegisterParams(name: name, password: password, role: role, branchCity: branchCity));
    return result.when(
      success: (_) {
        status = AuthStatus.unauthenticated;
        justRegistered = true;
        notifyListeners();
        return true;
      },
      failure: (f) {
        status = AuthStatus.error;
        errorMessage = f.message;
        notifyListeners();
        return false;
      },
    );
  }

  Future<Failure?> forgotPassword(String identifier) async {
    final result = await _forgotPasswordUseCase(ForgotPasswordParams(identifier: identifier));
    return result.when(success: (_) => null, failure: (f) => f);
  }

  Future<void> logout() async {
    await _logoutUseCase(const NoParams());
    employee = null;
    status = AuthStatus.unauthenticated;
    needsProfileSetup = false;
    needsFirstTimeOnboarding = false;
    needsMpinUnlock = false;
    notifyListeners();
  }
}
