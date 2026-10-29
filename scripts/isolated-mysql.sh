#!/bin/sh
set -eu
cd /mnt/e/workspace_work/项目/Talko
mkdir -p .tmp/mysql-packages .tmp/mysql-linux
cd .tmp/mysql-packages
apt-get download mysql-server-core-8.0 mysql-client-core-8.0 libaio1 libmecab2 libnuma1 libprotobuf-lite23 libevent-pthreads-2.1-7 libevent-core-2.1-7
for package in ./*.deb; do dpkg-deb -x "$package" ../mysql-linux; done
cd ..
export LD_LIBRARY_PATH="$PWD/mysql-linux/usr/lib/x86_64-linux-gnu"
mkdir -p /tmp/talko-dev-mysql
if [ ! -d /tmp/talko-dev-mysql/data/mysql ]; then
  ./mysql-linux/usr/sbin/mysqld --no-defaults --initialize-insecure --user=root --datadir=/tmp/talko-dev-mysql/data --basedir="$PWD/mysql-linux/usr"
fi
./mysql-linux/usr/sbin/mysqld --no-defaults --user=root --datadir=/tmp/talko-dev-mysql/data --secure-file-priv=NULL --mysqlx=OFF --port=3307 --bind-address=127.0.0.1 --socket=/tmp/talko-dev-mysql/mysql.sock --pid-file=/tmp/talko-dev-mysql/mysql.pid --log-error=/tmp/talko-dev-mysql/mysql.log --daemonize
./mysql-linux/usr/bin/mysql --socket=/tmp/talko-dev-mysql/mysql.sock -uroot -e "CREATE DATABASE IF NOT EXISTS talko CHARACTER SET utf8mb4; CREATE DATABASE IF NOT EXISTS talko_test CHARACTER SET utf8mb4; CREATE USER IF NOT EXISTS 'talko'@'%' IDENTIFIED BY 'talko_local'; GRANT ALL ON talko.* TO 'talko'@'%'; GRANT ALL ON talko_test.* TO 'talko'@'%'; FLUSH PRIVILEGES;"
