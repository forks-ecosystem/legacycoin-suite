eval "$(ssh-agent -s)"
ssh-add ~/.ssh/forks-ecosystem

cd /home/coin/legacycoin-miner

git config user.email "myshopper.club@gmail.com"
git config user.name "forks-ecosystem"

git add .
git commit -m "Initial commit: LegacyCoin Miner with Docker and web dashboard"
git remote add origin git@github.com:forks-ecosystem/legacycoin-miner.git 2>/dev/null
git branch -M main
git push -u origin main

ssh-agent -k
