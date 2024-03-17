# Action

0. Default (site.com)
1. Seon.io
2. Binlist.net
3. API

# bin data is in dump.sql

# Installation

```

sed -i '/^bind 127.0.0.1 ::1/cbind 0.0.0.0' /etc/redis/redis.conf
sed -i '/^supervised no/csupervised systemd' /etc/redis/redis.conf
sed -i '/^# replicaof */creplicaof global.site.com 6379' /etc/redis/redis.conf


service redis-server restart

ufw allow 6379
```

```
cat binupdatesubscriber.service > /lib/systemd/system/binupdatesubscriber.service
systemctl daemon-reload
systemctl enable binupdatesubscriber.service
service binupdatesubscriber start

```

## Set up Database

```
create database bin;
use bin;
create user "server"@"%" identified by "password";
GRANT ALL PRIVILEGES ON *.* TO 'server'@'%';
ALTER USER 'root'@'localhost' IDENTIFIED BY 'password';
flush privileges;
```

```
sudo nano /etc/mysql/mysql.conf.d/mysqld.cnf
max_connections = 1000000
sudo systemctl restart mysql



show variables like "max_connections";
```

# Db Schema

```
create table issuer ( id int(11) NOT NULL AUTO_INCREMENT  PRIMARY KEY,   name varchar(255),  headquarters varchar(255),  country varchar(255),  phone varchar(255),   website varchar(255),  email varchar(255),   created_at bigint,   updated_at   bigint ) ;

create table bins (start bigint primary key,end bigint,flag varchar(2),network varchar(100), type varchar(100),product_name varchar(100),issuer varchar(100),issuer_id int,country varchar(2),info varchar(100), updated_at bigint)

create table duplicatebins (start bigint ,end bigint,flag varchar(2),network varchar(100), type varchar(100),product_name varchar(100),issuer varchar(100),country varchar(2) , updated_at bigint)

create table updatebin (bin bigint,network varchar(200), type varchar(200),product_name varchar(200),issuer varchar(200),country varchar(2))

create table search_history (uuid BINARY(16) primary key, user_id BINARY(16), bin bigint ,ip bigint ,country varchar(2),state varchar(50),city varchar(50),cordinate POINT  action int,searched_at bigint)

create table global_stat (day_bin bigint , day_count bigint ,  week_bin bigint , week_count bigint , month_bin bigint , month_count bigint)

create table report (uuid BINARY(16) primary key, user_id BINARY(16), bin bigint ,ip bigint , network varchar(100), type varchar(100),product_name varchar(100),issuer varchar(100), country varchar(2),text varchar(500) ,reported_at bigint)

create table feedback (bin bigint primary key, likes INT DEFAULT 0 not null, dislike int default 0 not null );

create table feedback_history (uuid BINARY(16), user_id BINARY(16), bin bigint, ip bigint ,feedback_at bigint)

create table users  (user_id BINARY(16) primary key, email varchar(100) UNIQUE, name varchar(100),is_pro tinyint default 0 not null,pro_expire bigint default 0 not null, purchase_token varchar(100) default null,image varchar(500),country varchar(2),state varchar(50),city varchar(50),ip bigint,locate varchar(50),bins JSON not null default (JSON_ARRAY()),created_at bigint, updated_at bigint)


create table savedlist (uuid BINARY(16) primary key, user_id BINARY(16), name varchar(100), bins JSON not null default (JSON_ARRAY()), created_at bigint, updated_at bigint)

CREATE INDEX bin_searched_at on search_history (bin, searched_at)

create table ip (start bigint primary key, end bigint , country varchar(2), state varchar(100), city varchar(100))

create table asninfo (asn int primary key, name varchar(256), email varchar(100), phone varchar(100),orgname varchar(100), orgid varchar(100) )



CREATE TABLE setting ( `keypair` VARCHAR(255) UNIQUE,  `value` VARCHAR(9999), PRIMARY KEY (`keypair`) );

CREATE TABLE pending_transactions (
  payment_intent_id varchar(100) PRIMARY KEY,
  user_id binary(16),
  amount decimal(20, 5),
  created_at bigint
)


CREATE TABLE pending_paypal_transactions (
  order_id varchar(100) PRIMARY KEY,
  user_id binary(16),
  amount decimal(20, 5),
  created_at bigint
)

```

# Clean db

```
delete from bins where flag is not  null;
```
