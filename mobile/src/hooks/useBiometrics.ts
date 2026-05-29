import { useState, useEffect } from 'react';
import ReactNativeBiometrics from 'react-native-biometrics';

interface BiometricsHook {
  isBiometricAvailable: boolean;
  isLoading: boolean;
  authenticate: () => Promise<boolean>;
  initBiometrics: () => Promise<void>;
}

export const useBiometrics = (): BiometricsHook => {
  const [isBiometricAvailable, setIsBiometricAvailable] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  const rnBiometrics = new ReactNativeBiometrics({
    allowDeviceCredentials: true,
  });

  const initBiometrics = async () => {
    try {
      const biometryType = await rnBiometrics.isSensorAvailable();
      setIsBiometricAvailable(!!biometryType);
    } catch (error) {
      setIsBiometricAvailable(false);
    }
  };

  const authenticate = async (): Promise<boolean> => {
    setIsLoading(true);
    try {
      const { success } = await rnBiometrics.simplePrompt({
        promptMessage: 'Authenticate to continue',
        fallbackPromptMessage: 'Use biometric or passcode',
      });
      return success;
    } catch (error) {
      return false;
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    initBiometrics();
  }, []);

  return {
    isBiometricAvailable,
    isLoading,
    authenticate,
    initBiometrics,
  };
};
