import React, { useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  ActivityIndicator,
  Alert,
} from 'react-native';
import api from '../../services/api';

export default function TradeScreen({ route }) {
  const { assetId } = route.params || {};
  const [amount, setAmount] = useState('');
  const [price, setPrice] = useState('');
  const [type, setType] = useState('buy');
  const [loading, setLoading] = useState(false);

  const handleTrade = async () => {
    if (!amount || !price || !assetId) {
      Alert.alert('Error', 'Please fill in all fields');
      return;
    }

    setLoading(true);
    try {
      await api.post('/trades', {
        asset_id: assetId,
        amount: parseFloat(amount),
        price_per_unit: parseFloat(price),
        type: type,
      });

      Alert.alert('Success', `${type.charAt(0).toUpperCase() + type.slice(1)} order placed`);
      setAmount('');
      setPrice('');
    } catch (error) {
      Alert.alert('Error', error.response?.data?.message || 'Trade failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView style={styles.container}>
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Trade Type</Text>
        <View style={styles.tradeTypeContainer}>
          <TouchableOpacity
            style={[styles.tradeTypeButton, type === 'buy' && styles.tradeTypeActive]}
            onPress={() => setType('buy')}
          >
            <Text
              style={[
                styles.tradeTypeText,
                type === 'buy' && styles.tradeTypeTextActive,
              ]}
            >
              Buy
            </Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={[styles.tradeTypeButton, type === 'sell' && styles.tradeTypeActive]}
            onPress={() => setType('sell')}
          >
            <Text
              style={[
                styles.tradeTypeText,
                type === 'sell' && styles.tradeTypeTextActive,
              ]}
            >
              Sell
            </Text>
          </TouchableOpacity>
        </View>
      </View>

      <View style={styles.section}>
        <Text style={styles.label}>Amount</Text>
        <TextInput
          style={styles.input}
          placeholder="Enter amount"
          value={amount}
          onChangeText={setAmount}
          keyboardType="decimal-pad"
          editable={!loading}
        />

        <Text style={styles.label}>Price Per Unit</Text>
        <TextInput
          style={styles.input}
          placeholder="Enter price"
          value={price}
          onChangeText={setPrice}
          keyboardType="decimal-pad"
          editable={!loading}
        />

        {amount && price && (
          <View style={styles.totalSection}>
            <Text style={styles.totalLabel}>Total</Text>
            <Text style={styles.totalAmount}>
              ${(parseFloat(amount) * parseFloat(price)).toFixed(2)}
            </Text>
          </View>
        )}

        <TouchableOpacity
          style={[styles.submitButton, loading && styles.submitButtonDisabled]}
          onPress={handleTrade}
          disabled={loading}
        >
          {loading ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text style={styles.submitButtonText}>
              {type === 'buy' ? 'Place Buy Order' : 'Place Sell Order'}
            </Text>
          )}
        </TouchableOpacity>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8f8f8',
  },
  section: {
    backgroundColor: '#fff',
    padding: 16,
    marginVertical: 8,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 16,
    color: '#333',
  },
  tradeTypeContainer: {
    flexDirection: 'row',
    gap: 12,
  },
  tradeTypeButton: {
    flex: 1,
    padding: 12,
    borderRadius: 8,
    borderWidth: 2,
    borderColor: '#ddd',
    alignItems: 'center',
  },
  tradeTypeActive: {
    borderColor: '#007AFF',
    backgroundColor: '#e3f2fd',
  },
  tradeTypeText: {
    fontSize: 16,
    fontWeight: 'bold',
    color: '#999',
  },
  tradeTypeTextActive: {
    color: '#007AFF',
  },
  label: {
    fontSize: 14,
    fontWeight: 'bold',
    color: '#333',
    marginBottom: 8,
    marginTop: 12,
  },
  input: {
    borderWidth: 1,
    borderColor: '#ddd',
    padding: 12,
    borderRadius: 8,
    fontSize: 16,
  },
  totalSection: {
    marginTop: 16,
    padding: 12,
    backgroundColor: '#f0f0f0',
    borderRadius: 8,
  },
  totalLabel: {
    color: '#999',
    fontSize: 14,
  },
  totalAmount: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#333',
    marginTop: 4,
  },
  submitButton: {
    backgroundColor: '#007AFF',
    padding: 14,
    borderRadius: 8,
    alignItems: 'center',
    marginTop: 20,
  },
  submitButtonDisabled: {
    opacity: 0.6,
  },
  submitButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: 'bold',
  },
});
