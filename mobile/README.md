# KOR AssetForge Mobile App

Cross-platform mobile application for iOS and Android built with React Native.

## Features

- **User Authentication**: Email/password registration and login
- **Biometric Authentication**: Support for fingerprint and face recognition
- **Asset Marketplace**: Browse and search tokenized assets
- **Trading**: Buy and sell assets with real-time pricing
- **Portfolio Management**: Track holdings and performance
- **Stellar Wallet Integration**: Send and receive cryptocurrency
- **Push Notifications**: Real-time updates on trades and market changes

## Project Structure

```
mobile/
├── src/
│   ├── App.tsx                 # Main app component with navigation
│   ├── screens/
│   │   ├── auth/              # Login and registration screens
│   │   ├── marketplace/       # Browse and detail screens
│   │   ├── wallet/            # Wallet management
│   │   ├── trade/             # Trading interface
│   │   ├── portfolio/         # Portfolio management
│   │   └── profile/           # User profile and settings
│   ├── store/                 # Zustand state management
│   │   ├── authStore.ts       # Authentication state
│   │   └── walletStore.ts     # Wallet state
│   ├── services/              # API and external services
│   │   └── api.ts             # Axios API client
│   └── hooks/                 # Custom React hooks
│       └── useBiometrics.ts   # Biometric authentication
├── package.json
└── tsconfig.json
```

## Setup Instructions

### Prerequisites

- Node.js 16+ and npm/yarn
- React Native CLI
- Xcode (for iOS)
- Android Studio (for Android)

### Installation

```bash
cd mobile
npm install
```

### Environment Variables

Create a `.env` file in the mobile directory:

```
REACT_APP_API_URL=http://localhost:8080/api
```

### Running the App

**iOS:**
```bash
npm run ios
```

**Android:**
```bash
npm run android
```

**Development Server:**
```bash
npm start
```

## Building for Production

**iOS:**
```bash
npm run build:ios
```

**Android:**
```bash
npm run build:android
```

## Key Technologies

- **React Native**: Cross-platform mobile framework
- **React Navigation**: Navigation and routing
- **Zustand**: State management
- **Axios**: HTTP client
- **React Query**: Server state management
- **Stellar SDK**: Blockchain integration
- **React Native Biometrics**: Biometric authentication

## Security Features

- Biometric authentication support
- Secure token storage in device keychain
- JWT token-based API authentication
- Encrypted Stellar keypair storage

## Testing

```bash
npm test
```

## Linting and Formatting

```bash
npm run lint
npm run format
```

## Contributing

1. Create feature branches
2. Follow code style guidelines
3. Test thoroughly before submission
4. Update README with new features

## License

MIT
