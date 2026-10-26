eval "$(ssh-agent -s)"
ssh-add ~/.ssh/forks-ecosystem

cd /app/legacycoin-suite

git config user.email "myshopper.club@gmail.com"
git config user.name "forks-ecosystem"

git add .
git commit -m "update"
git remote add origin git@github.com:forks-ecosystem/legacycoin-suite.git 2>/dev/null
git branch -M main
git push -u origin main

ssh-agent -k
