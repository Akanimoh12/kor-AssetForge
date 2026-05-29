import React, { useEffect, useState } from 'react';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { View, ActivityIndicator } from 'react-native';
import Icon from 'react-native-vector-icons/Ionicons';

// Screens
import LoginScreen from './screens/auth/LoginScreen';
import RegisterScreen from './screens/auth/RegisterScreen';
import BrowseScreen from './screens/marketplace/BrowseScreen';
import DetailScreen from './screens/marketplace/DetailScreen';
import WalletScreen from './screens/wallet/WalletScreen';
import TradeScreen from './screens/trade/TradeScreen';
import PortfolioScreen from './screens/portfolio/PortfolioScreen';
import ProfileScreen from './screens/profile/ProfileScreen';

// Store
import { useAuthStore } from './store/authStore';
import { useBiometrics } from './hooks/useBiometrics';

const Stack = createNativeStackNavigator();
const Tab = createBottomTabNavigator();

// Auth Stack
function AuthStack() {
  return (
    <Stack.Navigator
      screenOptions={{
        headerShown: false,
        animationEnabled: true,
      }}
    >
      <Stack.Screen name="Login" component={LoginScreen} />
      <Stack.Screen name="Register" component={RegisterScreen} />
    </Stack.Navigator>
  );
}

// Main App Stack
function AppTabs() {
  return (
    <Tab.Navigator
      screenOptions={({ route }) => ({
        tabBarIcon: ({ focused, color, size }) => {
          let iconName = 'home';

          if (route.name === 'Browse') {
            iconName = focused ? 'search' : 'search-outline';
          } else if (route.name === 'Trade') {
            iconName = focused ? 'swap-horizontal' : 'swap-horizontal-outline';
          } else if (route.name === 'Portfolio') {
            iconName = focused ? 'briefcase' : 'briefcase-outline';
          } else if (route.name === 'Wallet') {
            iconName = focused ? 'wallet' : 'wallet-outline';
          } else if (route.name === 'Profile') {
            iconName = focused ? 'person' : 'person-outline';
          }

          return <Icon name={iconName} size={size} color={color} />;
        },
        tabBarActiveTintColor: '#007AFF',
        tabBarInactiveTintColor: '#999',
        headerShown: true,
        headerStyle: {
          backgroundColor: '#f8f8f8',
        },
      })}
    >
      <Tab.Screen
        name="Browse"
        component={BrowseStack}
        options={{
          title: 'Marketplace',
        }}
      />
      <Tab.Screen
        name="Trade"
        component={TradeScreen}
        options={{
          title: 'Trade',
        }}
      />
      <Tab.Screen
        name="Portfolio"
        component={PortfolioScreen}
        options={{
          title: 'Portfolio',
        }}
      />
      <Tab.Screen
        name="Wallet"
        component={WalletScreen}
        options={{
          title: 'Wallet',
        }}
      />
      <Tab.Screen
        name="Profile"
        component={ProfileScreen}
        options={{
          title: 'Profile',
        }}
      />
    </Tab.Navigator>
  );
}

// Browse Stack with nested navigation
function BrowseStack() {
  return (
    <Stack.Navigator
      screenOptions={{
        headerShown: true,
      }}
    >
      <Stack.Screen
        name="BrowseList"
        component={BrowseScreen}
        options={{
          title: 'Browse Assets',
        }}
      />
      <Stack.Screen
        name="Detail"
        component={DetailScreen}
        options={{
          title: 'Asset Details',
        }}
      />
    </Stack.Navigator>
  );
}

// Root Navigator
export default function App() {
  const { isAuthenticated, isLoading, validateToken } = useAuthStore();
  const [appReady, setAppReady] = useState(false);
  const { initBiometrics } = useBiometrics();

  useEffect(() => {
    const initializeApp = async () => {
      // Initialize biometric authentication
      await initBiometrics();

      // Validate existing token
      await validateToken();

      setAppReady(true);
    };

    initializeApp();
  }, []);

  if (!appReady) {
    return (
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        <ActivityIndicator size="large" color="#007AFF" />
      </View>
    );
  }

  return (
    <NavigationContainer>
      {isAuthenticated ? <AppTabs /> : <AuthStack />}
    </NavigationContainer>
  );
}
