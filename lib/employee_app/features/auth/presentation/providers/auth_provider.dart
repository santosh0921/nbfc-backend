import 'package:flutter/material.dart';
import 'package:local_auth/local_auth.dart';
import '../../../../core/constants/employee_app_module.dart';
import '../../../../core/error/failure.dart';
import '../../../../core/storage/local_storage_service.dart';
import '../../../../core/storage/mpin_service.dart';
import '../../../../core/usecase/usecase.dart';
import '../../../../../core/services/last_role_storage.dart';
import '../../domain/entities/employee_entity.dart';
import '../../domain/repositories/auth_repository.dart';
import '../../domain/usecases/login_usecase.dart';

enum AuthStatus { initial, authenticating, authenticated, unauthenticated, error }

class AuthProvider extends ChangeNotifier {
  AuthProvider({
    required LoginUseCase loginUseCase,
    required RegisterUseCase registerUseCase,
    required BiometricLoginUseCase biometricLoginUseCase,
    required LogoutUseCase logoutUseCase,
    required ForgotPasswordUseCase forgotPasswordUseCase,
    required AuthRepository authRepository,
    required LocalStorageService localStorage,
    required MpinService mpinService,
    this.module = EmployeeAppModule.verification,
  })  : _loginUseCase = loginUseCase,
        _registerUseCase = registerUseCase,
        _biometricLoginUseCase = biometricLoginUseCase,
        _logoutUseCase = logoutUseCase,
        _forgotPasswordUseCase = forgotPasswordUseCase,
        _authRepository = authRepository,
        _localStorage = localStorage,
        _mpinService = mpinService {
    _restoreSession();
  }

  final LoginUseCase _loginUseCase;
  final RegisterUseCase _registerUseCase;
  final BiometricLoginUseCase _biometricLoginUseCase;
  final LogoutUseCase _logoutUseCase;
  final ForgotPasswordUseCase _forgotPasswordUseCase;
  final AuthRepository _authRepository;
  final LocalStorageService _localStorage;
  final MpinService _mpinService;
  final _localAuth = LocalAuthentication();

  /// Which employee module (Verification or Recovery) this login screen
  /// was reached through — set once at construction time from the module
  /// the user picked on the role-selection screen. Used to gate login by
  /// the employee's actual [EmployeeRole] so e.g. a Collection Agent can't
  /// sign in through the Verification module and vice versa.
  final EmployeeAppModule module;

  /// The modules an employee with [role] is permitted to sign into.
  /// Field verification officers and supervisors work the Verification
  /// module (supervisors oversee verification officers); collection
  /// agents work the Recovery module.
  static bool _roleAllowedInModule(EmployeeRole role, EmployeeAppModule module) {
    switch (role) {
      case EmployeeRole.fieldVerificationOfficer:
      case EmployeeRole.supervisor:
        return module == EmployeeAppModule.verification;
      case EmployeeRole.collectionAgent:
        return module == EmployeeAppModule.recovery;
    }
  }

  static String _moduleName(EmployeeAppModule module) =>
      module == EmployeeAppModule.verification ? 'Verification' : 'Recovery';

  static String _mismatchMessage(EmployeeRole role, EmployeeAppModule attemptedModule) {
    final registeredModule = role == EmployeeRole.collectionAgent ? EmployeeAppModule.recovery : EmployeeAppModule.verification;
    return 'This account is registered for ${_moduleName(registeredModule)}, not ${_moduleName(attemptedModule)}. '
        'Please go back and select the correct role.';
  }

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
      if (!_roleAllowedInModule(session.role, module)) {
        // A session from a different module's login is on this device
        // (e.g. switched from Recovery to Verification role-select) —
        // don't silently let it through the wrong shell.
        await _authRepository.logout();
        status = AuthStatus.unauthenticated;
        notifyListeners();
        return;
      }
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

  Future<bool> get canUseBiometrics async {
    try {
      final supported = await _localAuth.isDeviceSupported();
      final canCheck = await _localAuth.canCheckBiometrics;
      return supported && canCheck;
    } catch (_) {
      return false;
    }
  }

  Future<void> login({required String name, required String password, bool remember = false}) async {
    status = AuthStatus.authenticating;
    errorMessage = null;
    notifyListeners();

    final result = await _loginUseCase(LoginParams(name: name, password: password, module: module.name));
    await result.when(
      success: (data) async {
        if (!_roleAllowedInModule(data.role, module)) {
          await _authRepository.logout();
          employee = null;
          status = AuthStatus.error;
          errorMessage = _mismatchMessage(data.role, module);
          return;
        }
        employee = data;
        status = AuthStatus.authenticated;
        rememberMe = remember;
        if (remember) {
          await _localStorage.setString(_kRememberedIdKey, name);
        } else {
          await _localStorage.remove(_kRememberedIdKey);
        }
        await LastRoleStorage.saveEmployee(module.name);
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

  Future<void> loginWithBiometrics() async {
    status = AuthStatus.authenticating;
    errorMessage = null;
    notifyListeners();

    try {
      final authenticated = await _localAuth.authenticate(
        localizedReason: 'Authenticate to access your ONEFIN employee account',
        options: const AuthenticationOptions(biometricOnly: true, stickyAuth: true),
      );
      if (!authenticated) {
        status = AuthStatus.unauthenticated;
        notifyListeners();
        return;
      }
    } catch (_) {
      status = AuthStatus.error;
      errorMessage = 'Biometric authentication is not available on this device.';
      notifyListeners();
      return;
    }

    final result = await _biometricLoginUseCase(const NoParams());
    await result.when(
      success: (data) async {
        if (!_roleAllowedInModule(data.role, module)) {
          await _authRepository.logout();
          employee = null;
          status = AuthStatus.error;
          errorMessage = _mismatchMessage(data.role, module);
          return;
        }
        employee = data;
        status = AuthStatus.authenticated;
        await LastRoleStorage.saveEmployee(module.name);
        // Biometric unlock re-admits an already-password-authenticated
        // session, so there is nothing left to gate behind the MPIN lock.
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
