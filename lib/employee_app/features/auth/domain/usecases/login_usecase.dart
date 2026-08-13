import '../../../../core/error/result.dart';
import '../../../../core/usecase/usecase.dart';
import '../entities/employee_entity.dart';
import '../repositories/auth_repository.dart';

class LoginParams {
  const LoginParams({required this.name, required this.password, required this.module});
  final String name;
  final String password;

  /// "verification" | "recovery" — the backend no longer consults this on
  /// login (an employee's actual role is stored server-side), it's kept
  /// purely for wire-format compatibility.
  final String module;
}

class LoginUseCase implements UseCase<EmployeeEntity, LoginParams> {
  LoginUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<EmployeeEntity>> call(LoginParams params) =>
      _repository.login(name: params.name, password: params.password, module: params.module);
}

class BiometricLoginUseCase implements UseCase<EmployeeEntity, NoParams> {
  BiometricLoginUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<EmployeeEntity>> call(NoParams params) => _repository.loginWithBiometrics();
}

class RegisterParams {
  const RegisterParams({required this.name, required this.password, required this.role, required this.branchCity});
  final String name;
  final String password;

  /// "verification" | "recovery" | "supervisor".
  final String role;
  final String branchCity;
}

class RegisterUseCase implements UseCase<void, RegisterParams> {
  RegisterUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<void>> call(RegisterParams params) =>
      _repository.register(name: params.name, password: params.password, role: params.role, branchCity: params.branchCity);
}

class LogoutUseCase implements UseCase<void, NoParams> {
  LogoutUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<void>> call(NoParams params) => _repository.logout();
}

class ForgotPasswordParams {
  const ForgotPasswordParams({required this.identifier});
  final String identifier;
}

class ForgotPasswordUseCase implements UseCase<void, ForgotPasswordParams> {
  ForgotPasswordUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<void>> call(ForgotPasswordParams params) =>
      _repository.forgotPassword(identifier: params.identifier);
}
