import '../../../../core/error/result.dart';
import '../../../../core/usecase/usecase.dart';
import '../entities/employee_entity.dart';
import '../repositories/auth_repository.dart';

class LoginParams {
  const LoginParams({required this.identifier, required this.password});
  final String identifier;
  final String password;
}

class LoginUseCase implements UseCase<EmployeeEntity, LoginParams> {
  LoginUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<EmployeeEntity>> call(LoginParams params) =>
      _repository.login(identifier: params.identifier, password: params.password);
}

class BiometricLoginUseCase implements UseCase<EmployeeEntity, NoParams> {
  BiometricLoginUseCase(this._repository);
  final AuthRepository _repository;

  @override
  Future<Result<EmployeeEntity>> call(NoParams params) => _repository.loginWithBiometrics();
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
