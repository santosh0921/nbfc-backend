/// Which employee module the ported app should land in after auth —
/// threaded from the outer NBFC role-selection screen, through
/// `EmployeeGateScreen` and `OnefinEmployeeApp`, into `AppRouter.build` so
/// the post-auth redirect fallthrough targets the right home route.
enum EmployeeAppModule { verification, recovery }
