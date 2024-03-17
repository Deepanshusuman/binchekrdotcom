#rm *.pem
#do only once in lifetime
#echo "Generating ca key and certificate"
#openssl req -x509 -newkey rsa:4096 -days 358000 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/C=NO/ST=/L=/O=CA/OU=Tools/CN=*.site.com/emailAddress=email@gmail.com"

echo "Generating Server self-signed certificate"
openssl req -newkey rsa:4096 -nodes -keyout server-key.pem -out server-req.pem -subj "/C=NO/ST=/L=/O=Server/OU=Tools/CN=*.site.com/emailAddress=email@gmail.com"

echo "Generating Server certificate"
openssl x509 -req -in server-req.pem -days 358000 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extfile conf.cnf

echo "Verify"
openssl verify -CAfile ca-cert.pem server-cert.pem

