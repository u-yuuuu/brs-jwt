import ssl
import requests
from requests.exceptions import SSLError, ConnectionError

CERTS_PATH = "C:/Users/user/Desktop/brs-main/certs"

BASE_URL = "https://localhost:8443"
ENDPOINTS = [
    "/api/mtls/health",
    "/api/mtls/protected"
]

def test_mtls():
    print("=" * 50)
    print("mTLS Test")
    print("=" * 50)
    
    session = requests.Session()
    
    session.verify = False
    
    cert_file = f"{CERTS_PATH}/client-cert.pem"
    key_file = f"{CERTS_PATH}/client-key.pem"
    
    import urllib3
    urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
    
    print(f"\n[INFO] Using certificate: {cert_file}")
    print(f"[INFO] Using key: {key_file}")
    
    for endpoint in ENDPOINTS:
        url = f"{BASE_URL}{endpoint}"
        print(f"\n1. Testing {endpoint}...")
        
        try:
            response = session.get(
                url,
                cert=(cert_file, key_file),
                timeout=5
            )
            
            if response.status_code == 200:
                print(f"   [OK] Status: {response.status_code}")
                print(f"   Response: {response.text}")
            else:
                print(f"   [FAIL] Status: {response.status_code}")
                print(f"   Response: {response.text}")
                
        except SSLError as e:
            print(f"   [FAIL] SSL Error: {e}")
        except ConnectionError as e:
            print(f"   [FAIL] Connection Error: {e}")
            print("   Make sure mTLS server is running on port 8443")
        except Exception as e:
            print(f"   [FAIL] Error: {e}")
    
    # Тест без сертификата 
    print("\n" + "=" * 50)
    print("2. Testing WITHOUT certificate (should fail)")
    print("=" * 50)
    
    try:
        response = session.get(f"{BASE_URL}/api/mtls/health", timeout=5)
        print(f"[WARN] Server accepted request without certificate!")
        print(f"   Status: {response.status_code}")
    except SSLError as e:
        print(f"[OK] Correctly rejected (SSL Error: {e})")
    except ConnectionError as e:
        print(f"[OK] Correctly rejected (Connection Error: {e})")
    except Exception as e:
        print(f"[INFO] Error: {e}")
    
    print("\n" + "=" * 50)
    print("Test Complete")
    print("=" * 50)

if __name__ == "__main__":
    test_mtls()