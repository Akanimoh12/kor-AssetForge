import React, { useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  ActivityIndicator,
} from 'react-native';
import { useQuery } from 'react-query';
import api from '../../services/api';

export default function PortfolioScreen() {
  const { data: portfolio, isLoading } = useQuery(
    ['portfolio'],
    () => api.get('/portfolio'),
    {
      select: (response) => response.data,
    }
  );

  if (isLoading) {
    return (
      <View style={styles.container}>
        <ActivityIndicator size="large" color="#007AFF" />
      </View>
    );
  }

  return (
    <ScrollView style={styles.container}>
      <View style={styles.summaryCard}>
        <Text style={styles.summaryLabel}>Total Portfolio Value</Text>
        <Text style={styles.summaryValue}>${portfolio?.total_value || '0.00'}</Text>
        <View style={styles.statsRow}>
          <View style={styles.stat}>
            <Text style={styles.statLabel}>Assets Held</Text>
            <Text style={styles.statValue}>{portfolio?.assets_count || 0}</Text>
          </View>
          <View style={styles.stat}>
            <Text style={styles.statLabel}>Total Gain/Loss</Text>
            <Text
              style={[
                styles.statValue,
                (portfolio?.total_gain || 0) >= 0
                  ? { color: '#4caf50' }
                  : { color: '#d32f2f' },
              ]}
            >
              {(portfolio?.total_gain || 0) >= 0 ? '+' : ''}${portfolio?.total_gain || '0.00'}
            </Text>
          </View>
        </View>
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Holdings</Text>
        {portfolio?.holdings?.map((holding) => (
          <View key={holding.asset_id} style={styles.holdingItem}>
            <View>
              <Text style={styles.assetName}>{holding.asset_name}</Text>
              <Text style={styles.assetSymbol}>{holding.symbol}</Text>
            </View>
            <View>
              <Text style={styles.amount}>{holding.quantity} units</Text>
              <Text style={styles.value}>${holding.current_value}</Text>
            </View>
          </View>
        ))}
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Performance</Text>
        <View style={styles.chartPlaceholder}>
          <Text style={styles.chartText}>Portfolio performance chart</Text>
          <Text style={styles.chartSubtext}>Coming soon</Text>
        </View>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8f8f8',
  },
  summaryCard: {
    backgroundColor: '#fff',
    padding: 20,
    margin: 12,
    borderRadius: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 3,
    elevation: 3,
  },
  summaryLabel: {
    color: '#999',
    fontSize: 14,
  },
  summaryValue: {
    fontSize: 32,
    fontWeight: 'bold',
    color: '#333',
    marginTop: 8,
  },
  statsRow: {
    flexDirection: 'row',
    marginTop: 16,
  },
  stat: {
    flex: 1,
  },
  statLabel: {
    color: '#999',
    fontSize: 12,
  },
  statValue: {
    fontSize: 18,
    fontWeight: 'bold',
    color: '#333',
    marginTop: 4,
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
  holdingItem: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#eee',
  },
  assetName: {
    fontSize: 14,
    fontWeight: 'bold',
    color: '#333',
  },
  assetSymbol: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
  },
  amount: {
    fontSize: 14,
    fontWeight: 'bold',
    color: '#333',
    textAlign: 'right',
  },
  value: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
    textAlign: 'right',
  },
  chartPlaceholder: {
    backgroundColor: '#f0f0f0',
    padding: 40,
    borderRadius: 8,
    alignItems: 'center',
  },
  chartText: {
    color: '#999',
    fontSize: 14,
  },
  chartSubtext: {
    color: '#999',
    fontSize: 12,
    marginTop: 4,
  },
});
