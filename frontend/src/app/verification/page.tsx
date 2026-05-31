"use client";

import { useState, useEffect, useCallback } from "react";
import { Header } from "@/components/Header";
import { WalletConnect } from "@/components/WalletConnect";
import { VerificationRequestForm } from "@/components/verification/VerificationRequestForm";
import { VerifierDashboard, VerificationRequest } from "@/components/verification/VerifierDashboard";
import { VerificationBadge } from "@/components/verification/VerificationBadge";
import { StellarWallet } from "@/lib/stellar";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent } from "@/components/ui/card";
import { ShieldCheck, ClipboardList } from "lucide-react";

export default function VerificationPage() {
  const [wallet, setWallet] = useState<StellarWallet | undefined>();
  const [requests, setRequests] = useState<VerificationRequest[]>([]);
  const [isVerifier, setIsVerifier] = useState(false);
  const api = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

  const loadRequests = useCallback(async () => {
    try {
      const res = await fetch(`${api}/api/verification/requests`);
      if (res.ok) setRequests(await res.json());
    } catch {
      // ignore
    }
  }, [api]);

  const checkVerifier = useCallback(async () => {
    if (!wallet) return;
    try {
      const res = await fetch(`${api}/api/verification/is-verifier/${wallet.publicKey}`);
      if (res.ok) setIsVerifier((await res.json()).is_verifier);
    } catch {
      // ignore
    }
  }, [wallet, api]);

  useEffect(() => {
    loadRequests();
  }, [loadRequests]);

  useEffect(() => {
    checkVerifier();
  }, [checkVerifier]);

  const myRequests = wallet
    ? requests.filter((r) => r.requester === wallet.publicKey)
    : [];

  return (
    <div className="min-h-screen bg-background">
      <Header wallet={wallet} />
      <main className="container mx-auto px-4 py-8 max-w-4xl">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">Asset Verification</h1>
            <p className="text-muted-foreground mt-1">
              Request verification for your assets or review pending requests as a verifier.
            </p>
          </div>
          {!wallet && (
            <WalletConnect
              onWalletConnected={(w) => setWallet(w)}
              onWalletDisconnected={() => setWallet(undefined)}
              wallet={wallet}
            />
          )}
        </div>

        <Tabs defaultValue={isVerifier ? "dashboard" : "request"}>
          <TabsList className="mb-6">
            <TabsTrigger value="request" className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4" />
              Request Verification
            </TabsTrigger>
            {isVerifier && (
              <TabsTrigger value="dashboard" className="flex items-center gap-2">
                <ClipboardList className="h-4 w-4" />
                Verifier Dashboard
              </TabsTrigger>
            )}
          </TabsList>

          <TabsContent value="request">
            <div className="space-y-6">
              {wallet ? (
                <VerificationRequestForm
                  wallet={wallet}
                  onSubmitted={loadRequests}
                />
              ) : (
                <Card>
                  <CardContent className="pt-6 text-center text-muted-foreground">
                    Connect your wallet to request asset verification.
                  </CardContent>
                </Card>
              )}

              {myRequests.length > 0 && (
                <div>
                  <h2 className="text-lg font-semibold mb-3">My Requests</h2>
                  <div className="space-y-2">
                    {myRequests.map((req) => (
                      <div
                        key={req.asset_id}
                        className="flex items-center justify-between p-3 rounded-lg border"
                      >
                        <span className="text-sm font-medium">Asset #{req.asset_id}</span>
                        <VerificationBadge
                          status={
                            req.status === "Approved"
                              ? "verified"
                              : req.status === "Pending"
                              ? "pending"
                              : "unverified"
                          }
                          size="sm"
                        />
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </TabsContent>

          {isVerifier && wallet && (
            <TabsContent value="dashboard">
              <VerifierDashboard
                wallet={wallet}
                requests={requests}
                onReviewed={loadRequests}
              />
            </TabsContent>
          )}
        </Tabs>
      </main>
    </div>
  );
}