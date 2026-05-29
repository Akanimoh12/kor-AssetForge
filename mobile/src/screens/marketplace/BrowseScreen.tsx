import React, { useEffect, useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  TextInput,
  ActivityIndicator,
} from 'react-native';
import { useQuery } from 'react-query';
import api from '../../services/api';

export default function BrowseScreen({ navigation }) {
  const [searchQuery, setSearchQuery] = useState('');
  const [assetType, setAssetType] = useState('');

  const { data, isLoading, error } = useQuery(
    ['assets', searchQuery, assetType],
    () =>
      api.get('/assets/search', {
        params: {
          q: searchQuery,
          asset_type: assetType,
          limit: 50,
          page: 1,
        },
      }),
    {
      select: (response) => response.data,
    }
  );

  const handleAssetPress = (asset) => {
    navigation.navigate('Detail', { assetId: asset.id });
  };

  const renderAssetItem = ({ item }) => (
    <TouchableOpacity
      style={styles.assetCard}
      onPress={() => handleAssetPress(item)}
    >
      <View style={styles.cardContent}>
        <Text style={styles.assetName}>{item.name}</Text>
        <Text style={styles.assetSymbol}>{item.symbol}</Text>
        <Text style={styles.assetType}>{item.asset_type}</Text>
      </View>
    </TouchableOpacity>
  );

  return (
    <View style={styles.container}>
      <TextInput
        style={styles.searchInput}
        placeholder="Search assets..."
        value={searchQuery}
        onChangeText={setSearchQuery}
        placeholderTextColor="#999"
      />

      {isLoading ? (
        <ActivityIndicator size="large" color="#007AFF" style={{ marginTop: 20 }} />
      ) : error ? (
        <Text style={styles.errorText}>Failed to load assets</Text>
      ) : (
        <FlatList
          data={data?.assets || []}
          renderItem={renderAssetItem}
          keyExtractor={(item) => item.id.toString()}
          contentContainerStyle={styles.listContent}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8f8f8',
  },
  searchInput: {
    backgroundColor: '#fff',
    borderRadius: 8,
    padding: 12,
    margin: 12,
    fontSize: 16,
    borderWidth: 1,
    borderColor: '#ddd',
  },
  listContent: {
    paddingHorizontal: 12,
    paddingBottom: 20,
  },
  assetCard: {
    backgroundColor: '#fff',
    borderRadius: 8,
    padding: 16,
    marginVertical: 8,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 3,
    elevation: 3,
  },
  cardContent: {
    flex: 1,
  },
  assetName: {
    fontSize: 18,
    fontWeight: 'bold',
    color: '#333',
  },
  assetSymbol: {
    fontSize: 14,
    color: '#007AFF',
    marginTop: 4,
  },
  assetType: {
    fontSize: 12,
    color: '#999',
    marginTop: 4,
  },
  errorText: {
    textAlign: 'center',
    color: '#d32f2f',
    marginTop: 20,
  },
});
