#include "Cache.hpp"
#include <string>
#include <vector>
#include <iostream>

int main(){ 
    std::vector<std::string> keys;
    std::vector<std::string> values;

    keys.push_back("K1");
    keys.push_back("K2");
    values.push_back("V1");
    values.push_back("V2");

    Cache cache = Cache(keys, values, 3);

    if (cache.size()!=2){
        std::cout<<"Expected size 2\n";
    }

    if(!cache.checkIfKeyExists("K1")){
        std::cout<<"K1 Should Exist!\n";
    }

    if (cache.checkIfKeyExists("K3")) {
		std::cout<<"K3 Should Not Exist!\n";
	}
    
    if(cache.getValueWithoutPositionChange("K1")!="V1"){
        std::cout<<"Expected V1 !\n";
    }

    cache.insertIntoCache("K3", "V3");
    if (cache.size() != 3) {
		std::cout<<"Expected Size 3\n";
	}

    if (!cache.checkIfKeyExists("K3")) {
		std::cout<<("key3 should exist");
	}

    cache.insertIntoCache("K4", "V4");

	if (cache.size() != 3) {
		std::cout<<"Expected Size 3\n";
	}

    if (cache.checkIfKeyExists("K1")) {
		std::cout<<("key1 should not exist");
	}


    std::vector<std::string> expectd = cache.evictLRElementFromCache();
    if (expectd[0] != "K2" || expectd[1] != "V2") {
		std::cout<<"Expected evicted (K2, V2)\n";
	}

    if (cache.size() != 2) {
		std::cout<<"Expected Size 2\n";
	}

    bool isPresent=false;
    std::string val = cache.getValueWithoutPositionChangeNew("K3",isPresent);
	if (!isPresent) {
		std::cout<<("key3 should be present");
	}
	if (val != "V3") {
		std::cout<<("Expected V3");
	}


    isPresent=false;
    std::string val2 = cache.getValueWithoutPositionChangeNew("K1",isPresent);
	if (isPresent) {
		std::cout<<("key1 should not be present");
	}
	if (val2 != "") {
		std::cout<<("Expected empty string");
	}
    
}