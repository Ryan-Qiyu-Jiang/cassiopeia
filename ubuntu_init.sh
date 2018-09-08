sudo apt-get update
sudo apt-get -y upgrade
sudo curl -O https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz
sudo tar -xvf go1.9.1.linux-amd64.tar.gz
sudo mv go /usr/local
sudo apt-get -y install git
sudo apt-get -y install vim
sudo apt-get -y install tmux

echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bash_profile
echo "export GOPATH=$HOME/projects" >> ~/.bash_profile
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/projects

mkdir projects
cd projects
mkdir src
cd src
git clone https://github.com/Ryan-Qiyu-Jiang/cassiopeia.git
go get github.com/spaolacci/murmur3
go install cassiopeia

