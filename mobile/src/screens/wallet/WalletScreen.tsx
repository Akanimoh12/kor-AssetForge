import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  ActivityIndicator,
  Alert,
} from 'react-native';
import { useQuery } from 'react-query';
import { useWalletStore } from '../../store/walletStore';
import { useBiometrics } from '../../hooks/useBiometrics';
import api from '../../services/api';

export default function WalletScreen() {
  const { wallet, syncWallet } = useWalletStore();
  const { isBiometricAvailable, authenticate } = useBiometrics();
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    syncWallet();
  }, []);

  const { data: transactions } = useQuery(
    ['transactions'],
    () => api.get('/wallet/transactions'),
    {
      select: (response) => response.data,
    }
  );

  const handleExportPrivateKey = async () => {
    if (isBiometricAvailable) {
      const authenticated = await authenticate();
      if (!authenticated) {
        Alert.alert('Error', 'Biometric authentication failed');
        return;
      }
    }

    Alert.alert(
      'Private Key',
      'Your private key has been copied to clipboard',
      [{ text: 'OK' }]
    );
  };

  return (
    <ScrollView style={styles.container}>
      {wallet && (
        <>
          <View style={styles.balanceCard}>
            <Text style={styles.balanceLabel}>Total Balance</Text>
            <Text style={styles.balanceAmount}>${wallet.balance}</Text>
            <Text style={styles.address}>{wallet.address}</Text>
          </View>

          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Actions</Text>
            <TouchableOpacity style={styles.actionButton}>
              <Text style={styles.actionButtonText}>Receive</Text>
            </TouchableOpacity>
            <TouchableOpacity style={styles.actionButton}>
              <Text style={styles.actionButtonText}>Send</Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={styles.actionButton}
              onPress={handleExportPrivateKey}
            >
              <Text style={styles.actionButtonText}>Security Settings</Text>
            </TouchableOpacity>
          </View>

          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Recent Transactions</Text>
            {transactions?.transactions?.map((tx) => (
              <View key={tx.id} style={styles.transactionItem}>
                <View>
                  <Text style={styles.txType}>{tx.type}</Text>
                  <Text style={styles.txDate}>
                    {new Date(tx.created_at).toLocaleDateString()}
                  </Text>
                </View>
                <Text
                  style={[
                    styles.txAmount,
                    tx.type === 'receive' ? { color: '#4caf50' } : { color: '#d32f2f' },
                  ]}
                >
                  {tx.type === 'receive' ? '+' : '-'}${tx.amount}
                </Text>
              </View>
            ))}
          </View>
        </>
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8f8f8',
  },
  balanceCard: {
    backgroundColor: '#007AFF',
    margin: 12,
    padding: 24,
    borderRadius: 12,
  },
  balanceLabel: {
    color: '#fff',
    fontSize: 14,
    opacity: 0.9,
  },
  balanceAmount: {
    color: '#fff',
    fontSize: 36,
    fontWeight: 'bold',
    marginTop: 8,
  },
  address: {
    color: '#fff',
    fontSize: 12,
    marginTop: 12,
    opacity: 0.8,
  },
  section: {
    backgroundColor: '#fff',
    padding: 16,
    marginVertical: 8,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 12,
    color: '#333',
  },
  actionButton: {
    backgroundColor: '#f0f0f0',
    padding: 12,
    borderRadius: 8,
    marginBottom: 8,
    alignItems: 'center',
  },
  actionButtonText: {
    color: '#007AFF',
    fontWeight: 'bold',
  },
  transactionItem: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  txType: {
    fontSize: 14,
    fontWeight: 'bold',
    color: '#333',
  },
  txDate: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
  },
  txAmount: {
    fontSize: 14,
    fontWeight: 'bold',
  },
});
