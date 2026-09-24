import AuthorizedApp, {
  shouldApplyScopedDogGuardResult,
  shouldRefreshSocialAfterDogGuardAction,
  socialRefreshPolicyForTab,
} from './AuthorizedApp';
import { CloseConfirmationController } from './components/CloseConfirmationDialog';

export { shouldApplyScopedDogGuardResult, shouldRefreshSocialAfterDogGuardAction, socialRefreshPolicyForTab };

function App() {
  return (
    <>
      <CloseConfirmationController />
      <AuthorizedApp />
    </>
  );
}

export default App;
