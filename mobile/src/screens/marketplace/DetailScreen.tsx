import React, { useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  ActivityIndicator,
} from 'react-native';
import { useQuery } from 'react-query';
import api from '../../services/api';

export default function DetailScreen({ route, navigation }) {
  const { assetId } = route.params;

  const { data: asset, isLoading } = useQuery(
    ['asset', assetId],
    () => api.get(`/assets/${assetId}`),
    {
      select: (response) => response.data,
    }
  );

  useEffect(() => {
    navigation.setOptions({
      title: asset?.name || 'Asset Details',
    });
  }, [asset]);

  if (isLoading) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="large" color="#007AFF" />
      </View>
    );
  }

  if (!asset) {
    return (
      <View style={styles.container}>
        <Text style={styles.errorText}>Asset not found</Text>
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <View style={styles.headerSection}>
        <Text style={styles.title}>{asset.name}</Text>
        <Text style={styles.symbol}>{asset.symbol}</Text>
        <Text style={styles.type}>{asset.asset_type}</Text>
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Description</Text>
        <Text style={styles.description}>{asset.description}</Text>
      </View>

      <View style={styles.section}>
        <View style={styles.statRow}>
          <Text style={styles.statLabel}>Total Supply</Text>
          <Text style={styles.statValue}>{asset.total_supply}</Text>
        </View>
        <View style={styles.statRow}>
          <Text style={styles.statLabel}>Fractions</Text>
          <Text style={styles.statValue}>{asset.fractions}</Text>
        </View>
        <View style={styles.statRow}>
          <Text style={styles.statLabel}>Status</Text>
          <Text style={[styles.statValue, asset.verified && { color: '#4caf50' }]}>
            {asset.verified ? 'Verified' : 'Pending'}
          </Text>
        </View>
      </View>

      <View style={styles.actionSection}>
        <TouchableOpacity
          style={styles.buyButton}
          onPress={() => navigation.navigate('Trade', { assetId })}
        >
          <Text style={styles.buttonText}>Buy Asset</Text>
        </TouchableOpacity>

        <TouchableOpacity style={styles.sellButton}>
          <Text style={styles.buttonTextSecondary}>View Listings</Text>
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
  headerSection: {
    backgroundColor: '#fff',
    padding: 20,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  title: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#333',
  },
  symbol: {
    fontSize: 16,
    color: '#007AFF',
    marginTop: 4,
  },
  type: {
    fontSize: 14,
    color: '#999',
    marginTop: 4,
  },
  section: {
    backgroundColor: '#fff',
    padding: 20,
    marginTop: 12,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 12,
    color: '#333',
  },
  description: {
    fontSize: 14,
    color: '#666',
    lineHeight: 20,
  },
  statRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  statLabel: {
    fontSize: 14,
    color: '#666',
  },
  statValue: {
    fontSize: 14,
    fontWeight: 'bold',
    color: '#333',
  },
  actionSection: {
    padding: 20,
    marginBottom: 20,
  },
  buyButton: {
    backgroundColor: '#007AFF',
    padding: 16,
    borderRadius: 8,
    alignItems: 'center',
    marginBottom: 12,
  },
  sellButton: {
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 8,
    alignItems: 'center',
    borderWidth: 1,
    borderColor: '#007AFF',
  },
  buttonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: 'bold',
  },
  buttonTextSecondary: {
    color: '#007AFF',
    fontSize: 16,
    fontWeight: 'bold',
  },
  errorText: {
    textAlign: 'center',
    color: '#d32f2f',
    marginTop: 20,
  },
});
