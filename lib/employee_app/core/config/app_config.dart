enum AppEnv { development, staging, production }

class AppConfig {
  const AppConfig._({required this.env, required this.apiBaseUrl, required this.useMockData});

  final AppEnv env;
  final String apiBaseUrl;
  final bool useMockData;

  static late AppConfig current;

  static void init(AppConfig config) {
    current = config;
  }

  // Overridable at build time with `--dart-define=API_BASE_URL=...` (same
  // pattern the customer app's ApiClient and the admin panel's ApiClient
  // both use) — defaults to the deployed Render backend when not
  // overridden, so login and every other call go to the real, always-on
  // backend by default. Without this override hook, there was no way to
  // point a build at a local backend at all: the constant below used to be
  // hardcoded to the Render URL regardless of any build flag passed.
  static const development = AppConfig._(
    env: AppEnv.development,
    apiBaseUrl: String.fromEnvironment(
      'API_BASE_URL',
      defaultValue: 'https://nbfc-backend-gr1t.onrender.com',
    ),
    useMockData: false,
  );

  static const production = AppConfig._(
    env: AppEnv.production,
    apiBaseUrl: 'https://api.onefin.com/v1',
    useMockData: false,
  );
}
